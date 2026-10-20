package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// AIExtractionCatalog 只读题材和股票主数据，为抽取模型提供受限词汇与精确 ID 解析。
type AIExtractionCatalog struct{ db *sql.DB }

func NewAIExtractionCatalog(db *sql.DB) *AIExtractionCatalog {
	return &AIExtractionCatalog{db: db}
}

var _ pipeline.Catalog = (*AIExtractionCatalog)(nil)

func (c *AIExtractionCatalog) Vocabulary(ctx context.Context) (guard.Vocabulary, error) {
	if c == nil || c.db == nil {
		return guard.Vocabulary{}, fmt.Errorf("题材目录未初始化")
	}
	rows, err := c.db.QueryContext(ctx, `SELECT name FROM addon_quant_theme
	  WHERE deleted_at IS NULL AND IFNULL(status, 1) = 1 AND name IS NOT NULL AND name <> '' ORDER BY id`)
	if err != nil {
		return guard.Vocabulary{}, fmt.Errorf("读取抽取题材词汇失败: %w", err)
	}
	defer rows.Close()
	out := guard.Vocabulary{
		Predicates: map[string]bool{"DISCLOSES_BUSINESS": true, "LISTED_IN_CATALOG": true},
		Themes:     map[string][]string{},
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return guard.Vocabulary{}, fmt.Errorf("扫描抽取题材词汇失败: %w", err)
		}
		key := guard.Normalize(name)
		if key == "" || utf8.RuneCountInString(key) > 64 {
			continue
		}
		out.Themes[key] = []string{key}
	}
	if err := rows.Err(); err != nil {
		return guard.Vocabulary{}, fmt.Errorf("读取抽取题材词汇失败: %w", err)
	}
	return out, nil
}

func (c *AIExtractionCatalog) StockByAlias(ctx context.Context, alias string) (pipeline.StockRef, bool, error) {
	if c == nil || c.db == nil {
		return pipeline.StockRef{}, false, fmt.Errorf("股票目录未初始化")
	}
	key := lookupKey(alias)
	if key == "" {
		return pipeline.StockRef{}, false, nil
	}
	rows, err := c.db.QueryContext(ctx, `SELECT id, IFNULL(ts_code, ''), IFNULL(name, '') FROM addon_quant_base_stock
	  WHERE deleted_at IS NULL AND ((name IS NOT NULL AND name = ?) OR (ts_code IS NOT NULL AND ts_code = ?))
	  ORDER BY id LIMIT 10`, alias, alias)
	if err != nil {
		return pipeline.StockRef{}, false, fmt.Errorf("精确解析股票名称失败: %w", err)
	}
	defer rows.Close()
	var found pipeline.StockRef
	matched := map[int64]bool{}
	for rows.Next() {
		var id int64
		var tsCode, name string
		if err := rows.Scan(&id, &tsCode, &name); err != nil {
			return pipeline.StockRef{}, false, fmt.Errorf("扫描股票目录失败: %w", err)
		}
		if lookupKey(name) != key && lookupKey(tsCode) != key {
			continue
		}
		if id <= 0 || tsCode == "" {
			continue
		}
		matched[id] = true
		found = pipeline.StockRef{ID: id, TsCode: tsCode}
		if len(matched) > 1 {
			return pipeline.StockRef{}, false, nil
		}
	}
	if err := rows.Err(); err != nil {
		return pipeline.StockRef{}, false, fmt.Errorf("读取股票目录失败: %w", err)
	}
	return found, len(matched) == 1, nil
}

func (c *AIExtractionCatalog) ThemeByKey(ctx context.Context, theme string) (pipeline.ThemeRef, bool, error) {
	if c == nil || c.db == nil {
		return pipeline.ThemeRef{}, false, fmt.Errorf("题材目录未初始化")
	}
	key := lookupKey(theme)
	if key == "" {
		return pipeline.ThemeRef{}, false, nil
	}
	rows, err := c.db.QueryContext(ctx, `SELECT id, IFNULL(name, '') FROM addon_quant_theme
	  WHERE deleted_at IS NULL AND IFNULL(status, 1) = 1 AND name IS NOT NULL AND name = ?
	  ORDER BY id LIMIT 10`, theme)
	if err != nil {
		return pipeline.ThemeRef{}, false, fmt.Errorf("精确解析题材名称失败: %w", err)
	}
	defer rows.Close()
	var found pipeline.ThemeRef
	matched := map[int64]bool{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return pipeline.ThemeRef{}, false, fmt.Errorf("扫描题材目录失败: %w", err)
		}
		if lookupKey(name) != key || id <= 0 {
			continue
		}
		matched[id] = true
		found = pipeline.ThemeRef{ID: id}
		if len(matched) > 1 {
			return pipeline.ThemeRef{}, false, nil
		}
	}
	if err := rows.Err(); err != nil {
		return pipeline.ThemeRef{}, false, fmt.Errorf("读取题材目录失败: %w", err)
	}
	return found, len(matched) == 1, nil
}

func lookupKey(value string) string { return strings.ToLower(guard.Normalize(value)) }

// AIExtractionRepo 负责持久化内部候选，不具有写 addon_quant_theme_stock 的方法。
type AIExtractionRepo struct{ db *sql.DB }

func NewAIExtractionRepo(db *sql.DB) *AIExtractionRepo { return &AIExtractionRepo{db: db} }

// SavePendingCandidates 原子保存待审候选。同快照、题材、股票和句子位置的重复结果幂等返回。
// 写入前在事务中重新检查快照正文、来源当前授权、句子编号和引用分段。
func (r *AIExtractionRepo) SavePendingCandidates(ctx context.Context, candidates []candidate.Candidate) ([]candidate.Candidate, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("抽取候选仓储未初始化")
	}
	if len(candidates) == 0 {
		return []candidate.Candidate{}, nil
	}
	for _, item := range candidates {
		if err := item.Validate(); err != nil {
			return nil, err
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("开启候选写入事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stored, err := savePendingCandidatesTx(ctx, tx, candidates)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交待审候选事务失败: %w", err)
	}
	return stored, nil
}

// savePendingCandidatesTx 复用同一事务，使抽取终态与候选队列原子提交。
func savePendingCandidatesTx(ctx context.Context, tx *sql.Tx, candidates []candidate.Candidate) ([]candidate.Candidate, error) {
	for _, item := range candidates {
		if err := item.Validate(); err != nil {
			return nil, err
		}
	}
	stored := make([]candidate.Candidate, 0, len(candidates))
	for _, item := range candidates {
		snapshot, err := lockAndLoadSnapshotForCandidate(ctx, tx, item)
		if err != nil {
			return nil, err
		}
		if err := candidate.ValidateAgainstSnapshot(item, snapshot); err != nil {
			return nil, err
		}
		if err := validateCandidateCatalog(ctx, tx, item); err != nil {
			return nil, err
		}
		identity, err := candidateEvidenceHash(item)
		if err != nil {
			return nil, fmt.Errorf("生成候选证据哈希失败: %w", err)
		}
		idsJSON, _ := json.Marshal(item.SentenceIDs)
		segmentsJSON, _ := json.Marshal(item.QuoteSegments)
		_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO tm_ai_extraction_candidates
		  (snapshot_id, claim_index, theme_id, theme_key, stock_id, entity_key, ts_code, predicate, source_type,
		   source_excerpt, sentence_ids, quote_segments, excerpt_trimmed, source_url, collected_at,
		   evidence_hash, provider, model, prompt_version, created_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(3))`,
			item.SnapshotID, item.ClaimIndex, item.ThemeID, item.ThemeKey, item.StockID, item.EntityKey, item.TsCode, item.Predicate, item.SourceType,
			item.SourceExcerpt, string(idsJSON), string(segmentsJSON), boolTinyInt(item.ExcerptTrimmed), item.SourceURL,
			item.CollectedAt.UTC().Format(databaseUTCLayout), identity[:], item.Provider, item.Model, item.PromptVersion)
		if err != nil {
			return nil, fmt.Errorf("保存待审抽取候选失败: %w", err)
		}
		existing, err := scanCandidate(tx.QueryRowContext(ctx, candidateSelect+` WHERE snapshot_id = ? AND theme_id = ? AND stock_id = ? AND evidence_hash = ?`,
			item.SnapshotID, item.ThemeID, item.StockID, identity[:]))
		if err != nil {
			return nil, fmt.Errorf("读取已保存抽取候选失败: %w", err)
		}
		if !sameCandidateEvidence(existing, item) {
			return nil, fmt.Errorf("候选唯一键命中但其证据字段不一致")
		}
		stored = append(stored, existing)
	}
	return stored, nil
}

const candidateSelect = `SELECT id, snapshot_id, claim_index, theme_id, theme_key, stock_id, entity_key, ts_code, predicate, source_type,
    source_excerpt, sentence_ids, quote_segments, excerpt_trimmed, source_url,
    DATE_FORMAT(collected_at, '` + databaseUTCSelect + `'), provider, model, prompt_version
  FROM tm_ai_extraction_candidates`

type candidateScanner interface{ Scan(...any) error }

func scanCandidate(row candidateScanner) (candidate.Candidate, error) {
	var item candidate.Candidate
	var idsJSON, segmentsJSON []byte
	var trimmed int8
	var collectedAtRaw string
	err := row.Scan(&item.ID, &item.SnapshotID, &item.ClaimIndex, &item.ThemeID, &item.ThemeKey, &item.StockID, &item.EntityKey, &item.TsCode,
		&item.Predicate, &item.SourceType, &item.SourceExcerpt, &idsJSON, &segmentsJSON, &trimmed,
		&item.SourceURL, &collectedAtRaw, &item.Provider, &item.Model, &item.PromptVersion)
	if err != nil {
		return candidate.Candidate{}, err
	}
	if err := json.Unmarshal(idsJSON, &item.SentenceIDs); err != nil {
		return candidate.Candidate{}, fmt.Errorf("解析候选句子编号失败: %w", err)
	}
	if err := json.Unmarshal(segmentsJSON, &item.QuoteSegments); err != nil {
		return candidate.Candidate{}, fmt.Errorf("解析候选原文分段失败: %w", err)
	}
	collectedAt, err := time.Parse(databaseUTCLayout, collectedAtRaw)
	if err != nil {
		return candidate.Candidate{}, fmt.Errorf("解析候选采集时点失败: %w", err)
	}
	item.CollectedAt = collectedAt.UTC()
	item.ExcerptTrimmed = trimmed == 1
	if err := item.Validate(); err != nil {
		return candidate.Candidate{}, fmt.Errorf("已存候选完整性校验失败: %w", err)
	}
	return item, nil
}

func lockAndLoadSnapshotForCandidate(ctx context.Context, tx *sql.Tx, item candidate.Candidate) (sources.Snapshot, error) {
	const query = `SELECT s.id, s.source_id, s.category_snapshot, s.url, s.content_hash, b.body_text, b.sentences_json,
	    s.license_tag_snapshot, s.license_ref_snapshot,
	    DATE_FORMAT(s.license_expires_snapshot, '` + databaseUTCSelect + `'), DATE_FORMAT(s.fetched_at, '` + databaseUTCSelect + `'),
	    src.domain, src.category, src.license_tag, IFNULL(src.license_ref, ''),
	    DATE_FORMAT(src.license_expires_at, '` + databaseUTCSelect + `'), src.enabled,
	    DATE_FORMAT(UTC_TIMESTAMP(3), '` + databaseUTCSelect + `')
	  FROM tm_ai_source_snapshots s
	  JOIN tm_ai_source_bodies b ON b.content_hash = s.body_ref
	  JOIN tm_ai_sources src ON src.id = s.source_id
	  WHERE s.id = ? FOR UPDATE`
	var snapshot sources.Snapshot
	var bodyJSON []byte
	var snapshotExpires, currentExpires sql.NullString
	var fetchedAtRaw, sourceDomain, sourceCategory, sourceLicenseTag, sourceLicenseRef, databaseNowRaw string
	var enabled int8
	err := tx.QueryRowContext(ctx, query, item.SnapshotID).Scan(
		&snapshot.ID, &snapshot.SourceID, &snapshot.SourceCategory, &snapshot.URL, &snapshot.Hash, &snapshot.Body, &bodyJSON,
		&snapshot.LicenseTag, &snapshot.LicenseRef, &snapshotExpires, &fetchedAtRaw,
		&sourceDomain, &sourceCategory, &sourceLicenseTag, &sourceLicenseRef, &currentExpires, &enabled, &databaseNowRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return sources.Snapshot{}, sources.ErrSourceUnavailable
	}
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("加载抽取候选来源快照失败: %w", err)
	}
	if snapshot.LicenseExpires, err = parseDatabaseUTC(snapshotExpires); err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析快照授权时点失败: %w", err)
	}
	if snapshot.FetchedAt, err = time.Parse(databaseUTCLayout, fetchedAtRaw); err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析快照采集时点失败: %w", err)
	}
	snapshot.FetchedAt = snapshot.FetchedAt.UTC()
	if err := json.Unmarshal(bodyJSON, &snapshot.Sentences); err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析快照句子索引失败: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return sources.Snapshot{}, fmt.Errorf("抽取候选来源快照完整性校验失败: %w", err)
	}
	databaseNow, err := time.Parse(databaseUTCLayout, databaseNowRaw)
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析来源授权复核时点失败: %w", err)
	}
	currentExpiry, err := parseDatabaseUTC(currentExpires)
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析当前来源授权到期时间失败: %w", err)
	}
	current := sources.Source{
		ID: snapshot.SourceID, Domain: sourceDomain, Category: sourceCategory,
		LicenseTag: sourceLicenseTag, LicenseRef: sourceLicenseRef,
		LicenseExpires: currentExpiry, Enabled: enabled == 1,
	}
	if !current.HasValidAuthorization(databaseNow.UTC()) || current.Category != snapshot.SourceCategory ||
		current.LicenseTag != snapshot.LicenseTag || current.LicenseRef != snapshot.LicenseRef ||
		!sameOptionalTime(current.LicenseExpires, snapshot.LicenseExpires) {
		return sources.Snapshot{}, sources.ErrUnauthorized
	}
	canonicalURL, err := sources.ValidateURL(current.Domain, snapshot.URL)
	if err != nil || canonicalURL != snapshot.URL {
		return sources.Snapshot{}, sources.ErrUnsafeURL
	}
	return snapshot, nil
}

func validateCandidateCatalog(ctx context.Context, tx *sql.Tx, item candidate.Candidate) error {
	var themeName string
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(name, '') FROM addon_quant_theme
	  WHERE id = ? AND deleted_at IS NULL AND IFNULL(status, 1) = 1`, item.ThemeID).Scan(&themeName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("候选题材已不存在或已停用")
		}
		return fmt.Errorf("复核候选题材失败: %w", err)
	}
	if lookupKey(themeName) != lookupKey(item.ThemeKey) {
		return fmt.Errorf("候选题材名称与当前主数据不一致")
	}
	var stockName, tsCode string
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(name, ''), IFNULL(ts_code, '') FROM addon_quant_base_stock
	  WHERE id = ? AND deleted_at IS NULL`, item.StockID).Scan(&stockName, &tsCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("候选股票已不存在")
		}
		return fmt.Errorf("复核候选股票失败: %w", err)
	}
	if tsCode != item.TsCode || (lookupKey(stockName) != lookupKey(item.EntityKey) && lookupKey(tsCode) != lookupKey(item.EntityKey)) {
		return fmt.Errorf("候选股票名称或代码与当前主数据不一致")
	}
	return nil
}

func candidateEvidenceHash(item candidate.Candidate) ([32]byte, error) {
	identity, err := json.Marshal(struct {
		SentenceIDs []int    `json:"sentence_ids"`
		Segments    []string `json:"segments"`
	}{SentenceIDs: item.SentenceIDs, Segments: item.QuoteSegments})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(identity), nil
}

func sameCandidateEvidence(stored, submitted candidate.Candidate) bool {
	return stored.SnapshotID == submitted.SnapshotID && stored.ThemeID == submitted.ThemeID &&
		stored.ThemeKey == submitted.ThemeKey && stored.StockID == submitted.StockID &&
		stored.EntityKey == submitted.EntityKey && stored.TsCode == submitted.TsCode &&
		stored.Predicate == submitted.Predicate && stored.SourceType == submitted.SourceType &&
		stored.SourceExcerpt == submitted.SourceExcerpt && stored.ExcerptTrimmed == submitted.ExcerptTrimmed &&
		stored.SourceURL == submitted.SourceURL && stored.CollectedAt.Equal(submitted.CollectedAt) &&
		stored.SameSentenceIDs(submitted) && slices.Equal(stored.QuoteSegments, submitted.QuoteSegments)
}

func boolTinyInt(value bool) int8 {
	if value {
		return 1
	}
	return 0
}
