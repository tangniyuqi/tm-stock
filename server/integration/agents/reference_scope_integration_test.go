//go:build integration

package agents_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type referenceExecutor struct {
	executor
	references atomic.Int32
}

func (e *referenceExecutor) ValidateReference(_ context.Context, r agents.Request) error {
	if !r.Valid() || !r.IsReference() {
		return agents.ErrInvalid
	}
	return nil
}
func (e *referenceExecutor) SubmitReference(_ context.Context, _ agents.Request) (agents.Acceptance, error) {
	e.references.Add(1)
	return agents.Acceptance{RemoteID: "reference_remote_01"}, nil
}

func TestIntegrationAgentReferenceScope(t *testing.T) {
	db := agentDB(t)
	repo := repository.NewAIAgentRepo(db)
	ctx := context.Background()
	executor := &referenceExecutor{}
	profile := agents.Profile{Digest: strings.Repeat("a", 64), MaxSubmissions: 2}
	s, err := service.NewAIAgentService(repo, executor, profile)
	if err != nil {
		t.Fatal(err)
	}
	req := agents.Request{RequestID: "reference_request_01", MemberID: 1, SubjectType: "theme", SubjectID: 1001, MaterialsDigest: strings.Repeat("b", 64)}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.Submit(ctx, 7, req)
			if err != nil && !errors.Is(err, agents.ErrUncertain) {
				t.Error("资料请求并发提交失败", err)
			}
			if err == nil && r.Request != req {
				t.Error("资料身份被替换")
			}
		}()
	}
	wg.Wait()
	r, err := s.Show(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Accepted || r.Request != req || executor.references.Load() != 1 || executor.calls.Load() != 0 {
		t.Fatal("资料请求没有独立登记/只提交一次")
	}
	for _, change := range []func(*agents.Request){func(r *agents.Request) { r.MemberID++ }, func(r *agents.Request) { r.SubjectID++ }, func(r *agents.Request) { r.MaterialsDigest = strings.Repeat("c", 64) }} {
		altered := req
		change(&altered)
		if _, err := s.Submit(ctx, 7, altered); !errors.Is(err, agents.ErrConflict) {
			t.Fatal("原请求被不同资料范围复用")
		}
	}
	for _, clause := range []string{"member_id=2", "subject_type='topic'", "subject_type='theme '", "subject_id=1002", "materials_digest=REPEAT('c',64)", "symbol='600001'", "actor_id=8", "request_id='changed_reference_01'", "request_id=CONCAT(request_id,' ')", "profile_digest=REPEAT('b',64)"} {
		_, err := db.ExecContext(ctx, `UPDATE tm_ai_agent_runs SET `+clause+` WHERE id=?`, r.ID)
		var me *mysql.MySQLError
		if !errors.As(err, &me) || me.Number != 1644 {
			t.Fatal("资料请求身份未被数据库保护", clause)
		}
	}
	// ASCII PAD SPACE 不能把带尾空格的对象类型视为合法 theme。
	if _, err := db.ExecContext(ctx, `INSERT INTO tm_ai_agent_runs(request_id,actor_id,symbol,member_id,subject_type,subject_id,materials_digest,profile_digest) VALUES('reference_padded_01',7,'',1,'theme ',1001,?,?)`, req.MaterialsDigest, profile.Digest); err == nil {
		t.Fatal("SQL 资料类型尾空格被接受")
	} else {
		var me *mysql.MySQLError
		if !errors.As(err, &me) || me.Number != 3819 {
			t.Fatal("SQL 资料类型未被 CHECK 明确拒绝")
		}
	}
	if err := repo.VerifySchema(ctx); err != nil {
		t.Fatal("新增结构启动探针失败", err)
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER tm_ai_agent_reference_scope_no_update`); err != nil {
		t.Fatal(err)
	}
	if err := repo.VerifySchema(ctx); err == nil {
		t.Fatal("缺资料不可变保护仍就绪")
	}
	agentReferenceDDL(t, db)
	if err := repo.VerifySchema(ctx); err != nil {
		t.Fatal("恢复保护后仍不可用", err)
	}
	latest, err := s.Show(ctx, 7, req.RequestID)
	if err != nil || latest != r {
		t.Fatal("迁移重放或启动探针修改原资料请求")
	}
	var profiles, runs, events int
	if db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_agent_profiles`).Scan(&profiles) != nil || db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_agent_runs`).Scan(&runs) != nil || db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_agent_events`).Scan(&events) != nil || profiles != 1 || runs != 1 || events != 2 {
		t.Fatal("探针未回滚或重放新增登记")
	}
}

func TestIntegrationAgentReferenceMigrationPreservesLegacy(t *testing.T) {
	db := agentDBWithScope(t, false)
	ctx := context.Background()
	profile := strings.Repeat("a", 64)
	old := agents.Record{ActorID: 7, Request: agents.Request{RequestID: "legacy_request_01", Symbol: "600001"}, ProfileDigest: profile, State: agents.Submitting}
	if _, err := db.ExecContext(ctx, `INSERT INTO tm_ai_agent_profiles(profile_digest,max_submissions,reserved_submissions) VALUES(?,2,1)`, profile); err != nil {
		t.Fatal(err)
	}
	result, err := db.ExecContext(ctx, `INSERT INTO tm_ai_agent_runs(request_id,actor_id,symbol,profile_digest) VALUES(?,7,'600001',?)`, old.Request.RequestID, profile)
	if err != nil {
		t.Fatal(err)
	}
	old.ID, err = result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO tm_ai_agent_events(run_id,seq,event,record_json) VALUES(?,1,'submit_reserved',?)`, old.ID, raw); err != nil {
		t.Fatal(err)
	}
	old.State, old.RemoteID = agents.Accepted, "legacy_remote_01"
	raw, err = json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tm_ai_agent_runs SET state='accepted',remote_id=?,event_seq=2 WHERE id=?`, old.RemoteID, old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO tm_ai_agent_events(run_id,seq,event,record_json) VALUES(?,2,'submitted',?)`, old.ID, raw); err != nil {
		t.Fatal(err)
	}
	var before, after string
	if db.QueryRowContext(ctx, `SELECT CAST(record_json AS CHAR) FROM tm_ai_agent_events WHERE run_id=? AND seq=2`, old.ID).Scan(&before) != nil {
		t.Fatal("历史事件不可读")
	}
	agentReferenceDDL(t, db)
	repo := repository.NewAIAgentRepo(db)
	got, err := repo.ByRequest(ctx, old.Request.RequestID)
	if err != nil || got != old || got.Request.IsReference() {
		t.Fatal("旧请求被升级或无法读取", err)
	}
	if db.QueryRowContext(ctx, `SELECT CAST(record_json AS CHAR) FROM tm_ai_agent_events WHERE run_id=? AND seq=2`, old.ID).Scan(&after) != nil || after != before {
		t.Fatal("历史审计被改写")
	}
	if err := repo.VerifySchema(ctx); err != nil {
		t.Fatal("迁移后原功能未通过启动核验", err)
	}
}
