package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func TestLocalWorkerChecksLifecycle(t *testing.T) {
	for _, command := range []string{"config", "audit", "plans"} {
		t.Run(command, func(t *testing.T) {
			modes := map[string][]string{
				"config": {"success", "verify_error", "cancel", "writer_error", "config_only_writer_error"},
				"audit":  {"success", "instance_error", "audit_error", "instance_cancel", "audit_cancel", "writer_error"},
				"plans":  {"success", "ready_error", "parent_cancel", "read_changed", "writer_error"},
			}[command]
			for _, mode := range modes {
				t.Run(mode, func(t *testing.T) {
					args := localCommandFiles(t)
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					out := &localCommandWriter{}
					if strings.HasSuffix(mode, "writer_error") {
						out.failAt = 1
					}
					calls := []string{}
					instance := func(c context.Context, db *sql.DB, id string) error {
						calls = append(calls, "instance")
						if c != ctx || db == nil || id != strings.Repeat("a", 32) {
							t.Fatal("实例核验参数被更改")
						}
						if mode == "cancel" || mode == "instance_cancel" {
							cancel()
							return c.Err()
						}
						if mode == "verify_error" || mode == "instance_error" {
							return errors.New("private_instance_error")
						}
						return nil
					}
					audit := func(c context.Context, db *sql.DB) error {
						calls = append(calls, "audit")
						if c != ctx || db == nil || strings.Join(calls, ",") != "instance,audit" {
							t.Fatal("审计先于实例核验或依赖丢失")
						}
						if mode == "audit_error" {
							return errors.New("private_audit_error")
						}
						if mode == "audit_cancel" {
							cancel()
							return c.Err()
						}
						return nil
					}
					ready := func(c context.Context, control *service.AILocalAdmissionControl) error {
						calls = append(calls, "ready")
						deadline, ok := c.Deadline()
						if control == nil || !ok || deadline.After(time.Now().Add(20*time.Second)) {
							t.Fatal("计划核验缺依赖或时限")
						}
						if mode == "ready_error" {
							return errors.New("private_ready_error")
						}
						if mode == "parent_cancel" {
							cancel()
							return c.Err()
						}
						return nil
					}
					want, code := 1, 1
					if mode == "success" {
						want = 0
					}
					if mode == "cancel" || strings.HasSuffix(mode, "_cancel") {
						want = 130
					}
					switch command {
					case "config":
						code = checkLocalConfigWith(ctx, args[:2], out, mode != "config_only_writer_error", instance)
						if mode == "config_only_writer_error" && len(calls) != 0 || mode != "config_only_writer_error" && strings.Join(calls, ",") != "instance" {
							t.Fatal("配置命令隐式调用或核验漏跑")
						}
					case "audit":
						code = checkLocalAuditWith(ctx, args[:2], out, instance, audit)
						expected := "instance,audit"
						if strings.HasPrefix(mode, "instance_") {
							expected = "instance"
						}
						if strings.Join(calls, ",") != expected {
							t.Fatal("实例拒绝后仍进入审计核验")
						}
					case "plans":
						if mode == "read_changed" {
							args[3] = "missing_local_test_plans"
						}
						code = checkLocalPlansWith(ctx, args[:4], out, ready)
						if mode == "read_changed" && len(calls) != 0 || mode != "read_changed" && strings.Join(calls, ",") != "ready" {
							t.Fatal("资料配置拒绝后仍进入执行核验")
						}
					}
					if code != want {
						t.Fatal("检查命令退出不符", code, want)
					}
					for _, secret := range []string{"private_", "fixture_secret", args[1], args[3], args[5]} {
						if strings.Contains(out.String(), secret) {
							t.Fatal("检查输出泄漏内部信息")
						}
					}
					if code != 0 && strings.Contains(out.String(), "核验通过") {
						t.Fatal("失败输出伪装成功")
					}
				})
			}
		})
	}
	ctx := context.Background()
	out := &localCommandWriter{}
	if checkLocalConfigWith(ctx, nil, out, true, nil) != 1 || checkLocalAuditWith(ctx, nil, out, nil, nil) != 1 || checkLocalPlansWith(ctx, nil, out, nil) != 1 {
		t.Fatal("缺少核验器仍启动")
	}
	if verifyLocalWorkerInstance(ctx, nil, "") == nil {
		t.Fatal("空依赖通过实例核验")
	}
}
