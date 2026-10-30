//go:build integration && !windows

package points_test

import (
	"os/exec"
	"syscall"
	"testing"
)

func startLocalSignalWorker(t *testing.T, e *workflowEnv, binary, dsn, runtimeFile, plansFile, executionFile string) (*localWorkerChild, func() error, string) {
	t.Helper()
	cmd := exec.CommandContext(e.ctx, binary, "run-local", "-config", runtimeFile, "-plans", plansFile, "-execution", executionFile)
	cmd.Env = localWorkerEnvironment(dsn, e.agentConfig.Bearer)
	child := startLocalWorkerCommand(t, cmd)
	return child, func() error { return cmd.Process.Signal(syscall.SIGTERM) }, ""
}
