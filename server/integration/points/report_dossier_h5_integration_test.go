//go:build integration && browser

package points_test

import (
	"os"
	"testing"
	"time"
)

func TestIntegrationAIReportDossierH5(t *testing.T) {
	origin := os.Getenv("TM_TEST_AI_H5_ORIGIN")
	if os.Getenv("TM_TEST_AI_H5_DOSSIER") != "local-fixture-only" {
		t.Fatal("档案H5需要显式本地验收入口")
	}
	if _, err := h5TestOrigin(origin); err != nil {
		t.Fatal("档案H5只接受完整本地回环源")
	}
	referenceFlag := os.Getenv("TM_TEST_AI_H5_DOSSIER_REFERENCES")
	if referenceFlag != "" && referenceFlag != "local-fixture-only" {
		t.Fatal("引用H5范围标记无效")
	}
	references := referenceFlag == "local-fixture-only"
	for _, key := range []string{"TM_TEST_AI_LIVE", "TM_TEST_AI_H5_LOCAL_API_WORKER", "TM_TEST_AI_H5_LOCAL_ASSISTANT", "TM_TEST_AI_H5_MATERIAL_QA", "TM_TEST_AI_H5_REPORT_QA"} {
		if os.Getenv(key) != "" {
			t.Fatal("档案H5不混用模型或其他专项")
		}
	}
	p := newDossierProcessFixture(t, 10*time.Minute, references)
	e := p.f.env
	g := newDossierH5Gateway(t, origin, p.base, p.login, func(enabled bool) error {
		_, err := e.db.ExecContext(e.ctx, `UPDATE tm_ai_dossier_publications SET enabled=? WHERE theme_id=1 AND version_no=1`, enabled)
		return err
	}, references)
	t.Log("专题档案实际H5入口（仅测试）：", g.server.URL+"/__test__/")
	select {
	case <-g.finish:
	case <-e.ctx.Done():
		t.Fatal("专题档案H5尚未完成，不能当作通过")
	}
	p.verifyNoGeneration(t)
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversations WHERE user_id=1`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversations WHERE user_id=2`) != 0 {
		t.Fatal("原会话创建或跨会员隔离不符")
	}
	if references {
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers WHERE user_id=1 AND theme_id=1 AND version_no=1 AND compare_version=2`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != 0 {
			t.Fatal("保存组合、账户隔离或原消息状态不符")
		}
		t.Log("会话引用保存/目录/重复读取及撤权恢复请求已核验，数据库唯一双版引用；刷新及布局操作仍以实际浏览器记录为证")
	}
	p.login.finish(t, e.ctx)
	t.Log("专题档案实际H5：原登录、真实目录/原文对照与撤权恢复请求已核验；无新增任务/账务/模型调用")
}
