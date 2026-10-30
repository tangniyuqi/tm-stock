//go:build integration && windows

package points_test

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

const localSignalHelperScope = "isolated-console-only"

func startLocalSignalWorker(t *testing.T, e *workflowEnv, binary, dsn, runtimeFile, plansFile, executionFile string) (*localWorkerChild, func() error, string) {
	t.Helper()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal("无法定位本轮信号测试程序")
	}
	cmd := exec.CommandContext(e.ctx, helper, "-test.run=^TestIntegrationAIReportLocalWorkerSignalHelper$")
	cmd.Env = append(localWorkerEnvironment(dsn, e.agentConfig.Bearer), "TM_TEST_LOCAL_SIGNAL_HELPER="+localSignalHelperScope,
		"TM_TEST_LOCAL_SIGNAL_BINARY="+binary, "TM_TEST_LOCAL_SIGNAL_RUNTIME="+runtimeFile,
		"TM_TEST_LOCAL_SIGNAL_PLANS="+plansFile, "TM_TEST_LOCAL_SIGNAL_EXECUTION="+executionFile)
	// 独立隐藏控制台；发送器和实际worker都不接入用户或父测试的控制台。
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010, HideWindow: true}
	child := startLocalWorkerCommand(t, cmd)
	return child, func() error { _, err := io.WriteString(child.input, "signal\n"); return err }, "LOCAL_SIGNAL_WORKER_EXIT:130"
}

// 仅由精确子进程入口调用；返回实际worker退出码，不能用测试宿主成功掩盖实际退出。
func TestIntegrationAIReportLocalWorkerSignalHelper(t *testing.T) {
	if os.Getenv("TM_TEST_LOCAL_SIGNAL_HELPER") != localSignalHelperScope {
		return
	}
	os.Exit(runLocalWindowsSignalHelper())
}

func runLocalWindowsSignalHelper() int {
	ctx, stop := context.WithTimeout(context.Background(), 70*time.Second)
	defer stop()
	cmd := exec.CommandContext(ctx, os.Getenv("TM_TEST_LOCAL_SIGNAL_BINARY"), "run-local", "-config", os.Getenv("TM_TEST_LOCAL_SIGNAL_RUNTIME"), "-plans", os.Getenv("TM_TEST_LOCAL_SIGNAL_PLANS"), "-execution", os.Getenv("TM_TEST_LOCAL_SIGNAL_EXECUTION"))
	cmd.Env = localWorkerEnvironment(os.Getenv("TM_DB_DSN"), os.Getenv("TM_AI_LOCAL_AGENT_BEARER"))
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	if err := cmd.Start(); err != nil {
		return 1
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waited := false
	defer func() {
		if !waited {
			// 只清理本函数自己启动的明确进程，绝不按名称或父PID清理其他程序。
			_ = cmd.Process.Kill()
			<-done
			_, _ = io.WriteString(os.Stdout, "LOCAL_SIGNAL_HELPER_CLEANED\n")
		}
	}()
	input := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(io.LimitReader(os.Stdin, 16)).ReadString('\n')
		input <- line
	}()
	select {
	case line := <-input:
		if line != "signal\n" {
			return 1
		}
		if err := signalOwnedLocalConsole(cmd.Process.Pid); err != nil {
			_, _ = io.WriteString(os.Stdout, "LOCAL_SIGNAL_HELPER_REFUSED:console\n")
			return 1
		}
		if _, err := io.WriteString(os.Stdout, "LOCAL_SIGNAL_HELPER_SENT\n"); err != nil {
			return 1
		}
	case <-done:
		waited = true
		return 1
	case <-ctx.Done():
		return 1
	}
	select {
	case err := <-done:
		waited = true
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 130 {
			return 1
		}
		if _, err := fmt.Fprintf(os.Stdout, "LOCAL_SIGNAL_WORKER_EXIT:%d\n", exited.ExitCode()); err != nil {
			return 1
		}
		return exited.ExitCode()
	case <-ctx.Done():
		return 1
	}
}

func TestLocalWorkerSignalTarget(t *testing.T) {
	// 拒绝零（广播）和自身目标；两例都在访问控制台/发送事件之前返回。
	for _, pid := range []int{0, os.Getpid()} {
		if signalOwnedLocalConsole(pid) == nil {
			t.Fatal("无效信号目标未拒绝")
		}
	}
}

func TestIntegrationAIReportLocalWorkerSignalControl(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	binary := filepath.Join(t.TempDir(), "local-ai-worker.exe")
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Minute)
	defer stop()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/ai-worker")
	build.Dir = filepath.Join("..", "..")
	if _, err := build.CombinedOutput(); err != nil {
		t.Fatal("构建实际worker失败")
	}
	verifyLocalWorkerSignal(t, binary, sql.LevelReadCommitted, 1, "invalid")
}

func signalOwnedLocalConsole(pid int) error {
	if pid <= 0 || pid == os.Getpid() {
		return errors.New("本轮信号目标无效")
	}
	kernel, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return err
	}
	defer kernel.Release()
	list, err := kernel.FindProc("GetConsoleProcessList")
	if err != nil {
		return err
	}
	ids := make([]uint32, 4)
	n, _, _ := list.Call(uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)))
	if n != 2 {
		return errors.New("本轮控制台未独立")
	}
	foundSelf, foundWorker := false, false
	for _, id := range ids[:int(n)] {
		foundSelf = foundSelf || id == uint32(os.Getpid())
		foundWorker = foundWorker || id == uint32(pid)
	}
	if !foundSelf || !foundWorker {
		return errors.New("本轮控制台进程不符")
	}
	event, err := kernel.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return err
	}
	ok, _, err := event.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	if ok == 0 {
		return err
	}
	return nil
}
