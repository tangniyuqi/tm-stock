//go:build integration

package points_test

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type officialSamplePassage struct {
	Key, Text          string
	StartByte, EndByte int
}
type officialSampleDocument struct {
	Key, Title, URL, NoticeURL, SourceType, RawSHA256, SourceMediaType, Extraction, Coverage string
	CollectedAt                                                                              int64
	RawBytes                                                                                 int
	Page                                                                                     *int
	Passages                                                                                 []officialSamplePassage
}
type officialSampleField struct{ Document, Passage, Text string }
type officialSampleCard struct {
	Key, Section, Document, Passage string
	Fields                          map[string]officialSampleField
}
type officialSampleTheme struct {
	Key, Name, Scope string
	Cards            []officialSampleCard
}
type officialSampleBundle struct {
	Schema, Classification, ReviewStatus string
	UsageBasis                           struct{ Kind, LawURL, Article, Scope string }
	Documents                            []officialSampleDocument
	Themes                               []officialSampleTheme
}

func loadOfficialSamples(t *testing.T) officialSampleBundle {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "eval", "research", "official-materials", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bundle officialSampleBundle
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&bundle) != nil || decoder.Decode(new(any)) != io.EOF || bundle.Schema != "tm-stock-official-research-samples-v1" || bundle.Classification != "real-official-passages-development-unreviewed" || bundle.ReviewStatus != "agent-source-checked-not-independent-human-gold" || len(bundle.Documents) != 4 || len(bundle.Themes) != 3 || bundle.UsageBasis.LawURL != "https://www.ncac.gov.cn/xxfb/flfg/flfg_532/202103/t20210309_50530.html" {
		t.Fatal("真实资料清单缺失或被误标为合成/人工金标")
	}
	return bundle
}

func importOfficialSampleSnapshots(t *testing.T, e *workflowEnv, bundle officialSampleBundle) (map[string]sources.Snapshot, map[string]dossier.SentenceRef, map[string]officialSampleDocument) {
	t.Helper()
	ctx := e.ctx
	sr := repository.NewAISourceRepo(e.db)
	registered := map[string]sources.Source{}
	snapshots := map[string]sources.Snapshot{}
	passages := map[string]dossier.SentenceRef{}
	documents := map[string]officialSampleDocument{}
	for _, doc := range bundle.Documents {
		u, err := url.Parse(doc.URL)
		digest, hashErr := hex.DecodeString(doc.RawSHA256)
		if err != nil || u.Scheme != "https" || (u.Host != "www.caac.gov.cn" && u.Host != "www.miit.gov.cn" && u.Host != "www.nea.gov.cn") || hashErr != nil || len(digest) != 32 || doc.RawBytes <= 0 || doc.SourceType != "announcement" || doc.Coverage != "selected-passages-not-full-document" || doc.CollectedAt <= 0 || doc.CollectedAt > time.Now().UnixMilli() || len(doc.Passages) == 0 {
			t.Fatal("正式原文的来源/捕获/节选范围不完整")
		}
		source, exists := registered[u.Host]
		if !exists {
			source, err = sr.RegisterSource(ctx, sources.Source{Domain: u.Host, Category: sources.CategoryAnnouncement, LicenseTag: "开发：正式机关文件及客观元数据；人工审核待办", LicenseRef: "dev-official-materials:copyright-law-art5:" + u.Host, MinInterval: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if err = sr.SetSourceEnabled(ctx, source.ID, true); err != nil {
				t.Fatal(err)
			}
			source, err = sr.SourceByID(ctx, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			registered[u.Host] = source
		}
		parts := []string{}
		for _, part := range doc.Passages {
			if part.Key == "" || part.StartByte < 0 || part.EndByte-part.StartByte != len([]byte(part.Text)) || strings.TrimSpace(part.Text) != part.Text {
				t.Fatal("原文提取定位不符")
			}
			parts = append(parts, part.Text)
		}
		snapshot, err := sources.BuildSnapshot(source, doc.URL, sources.MediaPlain, []byte(strings.Join(parts, "\n")), time.UnixMilli(doc.CollectedAt), 65536)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err = sr.SaveSnapshot(ctx, snapshot)
		if err != nil {
			t.Fatal(err)
		}
		snapshots[doc.Key], documents[doc.Key] = snapshot, doc
		for _, part := range doc.Passages {
			text := guard.Normalize(part.Text)
			found := false
			for _, sentence := range snapshot.Sentences {
				if sentence.Text == text {
					passages[doc.Key+":"+part.Key] = dossier.SentenceRef{SnapshotID: snapshot.ID, Hash: snapshot.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}
					found = true
					break
				}
			}
			if !found {
				t.Fatal("实际规范化句子与官方节选不符", doc.Key, part.Key)
			}
		}
	}
	return snapshots, passages, documents
}

// 真实原文由固定清单重放，不在CI反复访问机关网站，不借资料使用依据自动批准发布。
func TestIntegrationAIReportOfficialMaterialSamples(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	bundle := loadOfficialSamples(t)
	p := newDossierProcessFixture(t, 4*time.Minute)
	e := p.f.env
	ctx := e.ctx
	svc, err := service.NewAIDossierService(repository.NewAIDossierRepo(e.db, sql.LevelReadCommitted), dossierTestAuth{}, true)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, passages, documents := importOfficialSampleSnapshots(t, e, bundle)
	tokens, _ := localPasswordTokens(t, ctx, p.login, p.client)
	for i, theme := range bundle.Themes {
		t.Run(theme.Key, func(t *testing.T) {
			id := int64(1001 + i)
			e.mustExec(`INSERT INTO addon_quant_theme(id,name,status,description) VALUES(?,?,1,?)`, id, theme.Name, "真实机关文件开发资料样板；未独立人工审核。"+theme.Scope)
			draft := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: id, Version: 1, CreatedAt: time.Now().UTC().Truncate(time.Millisecond).UnixMilli(), Cards: []dossier.FactCard{}, Gaps: []dossier.Gap{{Key: "semantic_review", Section: "policy", Reason: "basis_unverified"}, {Key: "business_stage", Section: "progress", Reason: "stage_unverified"}}}
			used := map[string]bool{}
			for _, entry := range theme.Cards {
				ref, ok := passages[entry.Document+":"+entry.Passage]
				if !ok {
					t.Fatal("候选原文不存在")
				}
				unknown := dossier.Field{State: "unknown"}
				card := dossier.FactCard{Key: entry.Key, Section: entry.Section, Statement: ref, Subject: unknown, PublishedDate: unknown, EffectiveDate: unknown, Period: unknown, Scope: unknown, Stage: unknown, Basis: unknown, Unit: unknown}
				used[entry.Document] = true
				fields := map[string]*dossier.Field{"subject": &card.Subject, "publishedDate": &card.PublishedDate, "effectiveDate": &card.EffectiveDate, "period": &card.Period, "scope": &card.Scope, "stage": &card.Stage, "basis": &card.Basis, "unit": &card.Unit}
				for key, value := range entry.Fields {
					field, ok := fields[key]
					if !ok {
						t.Fatal("未知事实字段")
					}
					span, ok := passages[value.Document+":"+value.Passage]
					if !ok {
						t.Fatal("字段原文不存在")
					}
					snapshot := snapshots[value.Document]
					text := guard.Normalize(value.Text)
					start := strings.Index(snapshot.Body[span.StartByte:span.EndByte], text)
					if start < 0 || text == "" {
						t.Fatal("字段不是逐字原文")
					}
					span.StartByte += start
					span.EndByte = span.StartByte + len(text)
					*field = dossier.Field{State: "known", Ref: span}
					used[value.Document] = true
				}
				draft.Cards = append(draft.Cards, card)
			}
			digest, err := svc.StageInternal(ctx, draft)
			if err != nil {
				t.Fatal(err)
			}
			if again, err := svc.StageInternal(ctx, draft); err != nil || again != digest {
				t.Fatal("原候选重放发生变化", err)
			}
			// 单独的开发发布/会员授权，不冒充第三方许可或正式审核批准。
			e.mustExec(`INSERT INTO tm_ai_dossier_publications(theme_id,version_no,digest,approval_ref,enabled,valid_until,created_at,updated_at) VALUES(?,1,?,'dev-official-materials:not-production-reviewed',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, id, digest)
			e.mustExec(`INSERT INTO tm_ai_dossier_grants(user_id,theme_id,version_no,digest,approval_ref,enabled,valid_until,created_at,updated_at) VALUES(1,?,1,?,'dev-official-materials:member1-only',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, id, digest)
			path := fmt.Sprintf("/api/v1/ai/themes/%d/dossiers/1", id)
			read := func(bearer string, allowed bool) dto.AIDossierView {
				t.Helper()
				request, err := http.NewRequestWithContext(ctx, "GET", p.base+path, nil)
				if err != nil {
					t.Fatal(err)
				}
				if bearer != "" {
					request.Header.Set("Authorization", "Bearer "+bearer)
				}
				response, err := p.client.Do(request)
				if err != nil {
					t.Fatal("实际原API资料读取失败")
				}
				defer response.Body.Close()
				raw, err := io.ReadAll(io.LimitReader(response.Body, 262145))
				if err != nil || len(raw) > 262144 {
					t.Fatal("资料响应无效")
				}
				var envelope struct {
					Code int
					Data json.RawMessage
				}
				if json.Unmarshal(raw, &envelope) != nil || response.Header.Get("Cache-Control") != "no-store" {
					t.Fatal("资料响应契约错误")
				}
				if !allowed {
					if response.StatusCode == 200 || envelope.Code == 0 || len(envelope.Data) > 0 && string(envelope.Data) != "null" {
						t.Fatal("拒绝后仍返回原文/数量")
					}
					return dto.AIDossierView{}
				}
				var view dto.AIDossierView
				if response.StatusCode != 200 || envelope.Code != 0 || json.Unmarshal(envelope.Data, &view) != nil || view.ThemeID != id || len(view.Versions) != 1 || len(view.Versions[0].Cards) != len(theme.Cards) {
					t.Fatal("真实开发样板读取不符", response.StatusCode)
				}
				return view
			}
			for repeat := 0; repeat < 2; repeat++ {
				view := read(tokens[0], true)
				expectedCards := map[string]officialSampleCard{}
				for _, entry := range theme.Cards {
					expectedCards[entry.Key] = entry
				}
				for _, card := range view.Versions[0].Cards {
					entry, exists := expectedCards[card.Key]
					if !exists || card.Quote.SourceURL != documents[entry.Document].URL || card.Quote.CollectedAt != documents[entry.Document].CollectedAt || len(card.Fields) != 8 {
						t.Fatal("出处、采集时间或字段缺失")
					}
					delete(expectedCards, card.Key)
					for _, field := range card.Fields {
						expected, known := entry.Fields[field.Key]
						if known {
							if field.State != "known" || field.Quote == nil || field.Quote.Text != guard.Normalize(expected.Text) {
								t.Fatal("字段原文被改写")
							}
						} else if field.State != "unknown" || field.Quote != nil {
							t.Fatal("未知阶段、日期或单位被补成事实")
						}
					}
				}
				if len(expectedCards) != 0 {
					t.Fatal("候选卡片缺失或重复")
				}
			}
			read(tokens[1], false)
			read("", false)
			for _, table := range []string{"tm_ai_dossier_publications", "tm_ai_dossier_grants"} {
				if table == "tm_ai_dossier_publications" {
					e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=0 WHERE theme_id=?`, id)
				} else {
					e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=0 WHERE theme_id=?`, id)
				}
				read(tokens[0], false)
				if table == "tm_ai_dossier_publications" {
					e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=1 WHERE theme_id=?`, id)
				} else {
					e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=1 WHERE theme_id=?`, id)
				}
				read(tokens[0], true)
			}
			for key := range used {
				sourceID := snapshots[key].SourceID
				e.mustExec(`UPDATE tm_ai_sources SET enabled=0 WHERE id=?`, sourceID)
				read(tokens[0], false)
				e.mustExec(`UPDATE tm_ai_sources SET enabled=1 WHERE id=?`, sourceID)
				read(tokens[0], true)
			}
			p.verifyNoGeneration(t)
		})
	}
	p.login.finishAs(t, ctx, false)
}
