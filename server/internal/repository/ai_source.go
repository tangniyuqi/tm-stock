package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const (
	databaseUTCLayout = "2006-01-02 15:04:05.000000"
	databaseUTCSelect = "%Y-%m-%d %H:%i:%s.%f"
)

// AISourceRepo 负责 AI 资料来源和只增不改的快照存储。
type AISourceRepo struct {
	db *sql.DB
}

type sourceSQL interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// NewAISourceRepo 构造来源仓储。
func NewAISourceRepo(db *sql.DB) *AISourceRepo { return &AISourceRepo{db: db} }

// SourceByID 按编号读取登记来源，包括已停用记录供运营核对。
func (r *AISourceRepo) SourceByID(ctx context.Context, id int64) (sources.Source, error) {
	return sourceByID(ctx, r.db, id, false)
}

func sourceByID(ctx context.Context, db sourceSQL, id int64, lock bool) (sources.Source, error) {
	mode := sourceUnlocked
	if lock {
		mode = sourceExclusive
	}
	return sourceByIDWithMode(ctx, db, id, mode)
}

type sourceLockMode int

const (
	sourceUnlocked sourceLockMode = iota
	sourceExclusive
	sourceShared
)

func sourceByIDWithMode(ctx context.Context, db sourceSQL, id int64, mode sourceLockMode) (sources.Source, error) {
	if id <= 0 {
		return sources.Source{}, sources.ErrSourceUnavailable
	}
	const selectSource = `SELECT id, domain, category, license_tag, IFNULL(license_ref,''),
	    DATE_FORMAT(license_expires_at, '` + databaseUTCSelect + `'), min_interval_ms, enabled
      FROM tm_ai_sources WHERE id = ?`
	query := selectSource
	switch mode {
	case sourceUnlocked:
	case sourceExclusive:
		query = selectSource + ` FOR UPDATE`
	case sourceShared:
		query = selectSource + ` FOR SHARE`
	default:
		return sources.Source{}, sources.ErrSourceUnavailable
	}
	var source sources.Source
	var expires sql.NullString
	var intervalMS int64
	var enabled int8
	if err := db.QueryRowContext(ctx, query, id).Scan(
		&source.ID, &source.Domain, &source.Category, &source.LicenseTag, &source.LicenseRef,
		&expires, &intervalMS, &enabled,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sources.Source{}, sources.ErrSourceUnavailable
		}
		return sources.Source{}, fmt.Errorf("读取资料来源失败: %w", err)
	}
	expiresAt, err := parseDatabaseUTC(expires)
	if err != nil {
		return sources.Source{}, fmt.Errorf("解析来源授权到期时间失败: %w", err)
	}
	source.LicenseExpires = expiresAt
	source.MinInterval = time.Duration(intervalMS) * time.Millisecond
	source.Enabled = enabled == 1
	// 读取历史登记不等于授权使用。到期来源仍须可供内部核对/停用，使用方另行复核授权。
	wasEnabled := source.Enabled
	source.Enabled = false
	normalized, err := source.NormalizeRegistration()
	if err != nil {
		return sources.Source{}, fmt.Errorf("资料来源登记数据无效: %w", err)
	}
	normalized.Enabled = wasEnabled
	return normalized, nil
}

// RegisterSource 新增一个默认关闭的来源。
func (r *AISourceRepo) RegisterSource(ctx context.Context, source sources.Source) (sources.Source, error) {
	return registerSource(ctx, r.db, source)
}

func registerSource(ctx context.Context, db sourceSQL, source sources.Source) (sources.Source, error) {
	normalized, err := source.NormalizeRegistration()
	if err != nil {
		return sources.Source{}, err
	}
	if normalized.ID != 0 {
		return sources.Source{}, fmt.Errorf("新来源不能指定编号")
	}
	intervalMS := normalized.MinInterval.Milliseconds()
	if intervalMS < 1000 {
		intervalMS = 1000
	}
	if intervalMS > int64(^uint32(0)) {
		return sources.Source{}, fmt.Errorf("来源访问间隔超过数据库字段上限")
	}
	const query = `INSERT INTO tm_ai_sources
      (domain, category, license_tag, license_ref, license_expires_at, min_interval_ms, enabled, created_at, updated_at)
      VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, 0, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))`
	result, err := db.ExecContext(ctx, query, normalized.Domain, normalized.Category,
		normalized.LicenseTag, normalized.LicenseRef, nullableTime(normalized.LicenseExpires), intervalMS)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return sources.Source{}, fmt.Errorf("该来源域名已登记")
		}
		return sources.Source{}, fmt.Errorf("登记资料来源失败: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return sources.Source{}, fmt.Errorf("读取新来源编号失败: %w", err)
	}
	normalized.ID = id
	normalized.Enabled = false
	normalized.MinInterval = time.Duration(intervalMS) * time.Millisecond
	return normalized, nil
}

// SetSourceEnabled 启用来源前在数据库条件中再次确认书面授权存在且未过期。
func (r *AISourceRepo) SetSourceEnabled(ctx context.Context, id int64, enabled bool) error {
	return setSourceEnabled(ctx, r.db, id, enabled)
}

func setSourceEnabled(ctx context.Context, db sourceSQL, id int64, enabled bool) error {
	if id <= 0 {
		return sources.ErrSourceUnavailable
	}
	if !enabled {
		result, err := db.ExecContext(ctx,
			`UPDATE tm_ai_sources SET enabled = 0,
			  updated_at = GREATEST(UTC_TIMESTAMP(3), updated_at + INTERVAL 1000 MICROSECOND) WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("停用资料来源失败: %w", err)
		}
		return affectedOrUnavailable(result)
	}
	source, err := sourceByID(ctx, db, id, false)
	if err != nil {
		return err
	}
	source.Enabled = true
	if _, err := source.NormalizeRegistration(); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE tm_ai_sources
	    SET enabled = 1, updated_at = GREATEST(UTC_TIMESTAMP(3), updated_at + INTERVAL 1000 MICROSECOND)
	  WHERE id = ? AND license_tag <> '' AND license_ref IS NOT NULL AND license_ref <> ''
	    AND (license_expires_at IS NULL OR license_expires_at > UTC_TIMESTAMP(3))`, id)
	if err != nil {
		return fmt.Errorf("启用资料来源失败: %w", err)
	}
	if err := affectedOrUnavailable(result); err != nil {
		return fmt.Errorf("%w: 来源不存在或授权无效", sources.ErrUnauthorized)
	}
	return nil
}

// ValidateEnabledSources 用于生产服务启动检查，任何已启用但授权缺失或过期的来源都会阻止启动。
func (r *AISourceRepo) ValidateEnabledSources(ctx context.Context) error {
	const query = `SELECT id, domain, category, license_tag, IFNULL(license_ref,''),
	    DATE_FORMAT(license_expires_at, '` + databaseUTCSelect + `'), min_interval_ms, enabled,
	    DATE_FORMAT(UTC_TIMESTAMP(3), '` + databaseUTCSelect + `')
	  FROM tm_ai_sources WHERE enabled = 1 ORDER BY id`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("检查资料来源授权失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var source sources.Source
		var expires sql.NullString
		var intervalMS int64
		var enabled int8
		var databaseNowRaw string
		if err := rows.Scan(&source.ID, &source.Domain, &source.Category, &source.LicenseTag,
			&source.LicenseRef, &expires, &intervalMS, &enabled, &databaseNowRaw); err != nil {
			return fmt.Errorf("读取启用来源授权失败: %w", err)
		}
		if source.LicenseExpires, err = parseDatabaseUTC(expires); err != nil {
			return fmt.Errorf("解析来源授权到期时间失败: %w", err)
		}
		databaseNow, err := time.Parse(databaseUTCLayout, databaseNowRaw)
		if err != nil {
			return fmt.Errorf("解析数据库授权校验时点失败: %w", err)
		}
		source.MinInterval = time.Duration(intervalMS) * time.Millisecond
		source.Enabled = enabled == 1
		if _, err := source.NormalizeRegistration(); err != nil || !source.HasValidAuthorization(databaseNow.UTC()) {
			return fmt.Errorf("启用来源编号 %d 的授权或登记字段无效: %w", source.ID, sources.ErrUnauthorized)
		}
	}
	return rows.Err()
}

// SaveSnapshot 插入不可变快照；同一来源、链接与内容的重复采集返回原快照编号。
func (r *AISourceRepo) SaveSnapshot(ctx context.Context, snapshot sources.Snapshot) (sources.Snapshot, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("开启快照事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 提交后回滚返回 ErrTxDone，失败路径释放事务。
	stored, err := saveSnapshotTx(ctx, tx, snapshot)
	if err != nil {
		return sources.Snapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return sources.Snapshot{}, fmt.Errorf("提交资料快照失败: %w", err)
	}
	return stored, nil
}

func saveSnapshotTx(ctx context.Context, tx *sql.Tx, snapshot sources.Snapshot) (sources.Snapshot, error) {
	if err := snapshot.Validate(); err != nil {
		return sources.Snapshot{}, err
	}
	urlHash := sha256.Sum256([]byte(snapshot.URL))
	serializedSentences, err := json.Marshal(snapshot.Sentences)
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("序列化快照句子索引失败: %w", err)
	}
	var current sources.Source
	var expires sql.NullString
	var intervalMS int64
	var enabled int8
	var databaseNowRaw string
	err = tx.QueryRowContext(ctx, `SELECT id, domain, category, license_tag, IFNULL(license_ref,''),
	    DATE_FORMAT(license_expires_at, '`+databaseUTCSelect+`'), min_interval_ms, enabled,
	    DATE_FORMAT(UTC_TIMESTAMP(3), '`+databaseUTCSelect+`')
	  FROM tm_ai_sources WHERE id = ? FOR UPDATE`, snapshot.SourceID).Scan(
		&current.ID, &current.Domain, &current.Category, &current.LicenseTag, &current.LicenseRef,
		&expires, &intervalMS, &enabled, &databaseNowRaw,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sources.Snapshot{}, sources.ErrSourceUnavailable
		}
		return sources.Snapshot{}, fmt.Errorf("复核快照来源失败: %w", err)
	}
	if current.LicenseExpires, err = parseDatabaseUTC(expires); err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析来源授权到期时间失败: %w", err)
	}
	databaseNow, err := time.Parse(databaseUTCLayout, databaseNowRaw)
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析数据库授权校验时点失败: %w", err)
	}
	current.Enabled = enabled == 1
	current.MinInterval = time.Duration(intervalMS) * time.Millisecond
	current, err = current.NormalizeRegistration()
	if err != nil || !current.HasValidAuthorization(databaseNow.UTC()) || current.Category != snapshot.SourceCategory || current.LicenseTag != snapshot.LicenseTag || current.LicenseRef != snapshot.LicenseRef || !sameOptionalTime(current.LicenseExpires, snapshot.LicenseExpires) {
		return sources.Snapshot{}, sources.ErrUnauthorized
	}
	canonicalURL, err := sources.ValidateURL(current.Domain, snapshot.URL)
	if err != nil || canonicalURL != snapshot.URL {
		return sources.Snapshot{}, sources.ErrUnsafeURL
	}

	const findExisting = `SELECT s.id, s.source_id, s.category_snapshot, s.url, s.content_hash, b.body_text, b.sentences_json,
	    s.license_tag_snapshot, s.license_ref_snapshot,
	    DATE_FORMAT(s.license_expires_snapshot, '` + databaseUTCSelect + `'), DATE_FORMAT(s.fetched_at, '` + databaseUTCSelect + `')
	  FROM tm_ai_source_snapshots s JOIN tm_ai_source_bodies b ON b.content_hash = s.body_ref
	  WHERE s.source_id = ? AND s.url_hash = ? AND s.content_hash = ?`
	existing, err := scanSnapshot(tx.QueryRowContext(ctx, findExisting, snapshot.SourceID, urlHash[:], snapshot.Hash))
	if err == nil {
		if existing.Body != snapshot.Body || !slices.Equal(existing.Sentences, snapshot.Sentences) {
			return sources.Snapshot{}, fmt.Errorf("同一内容哈希对应的已存正文不一致")
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return sources.Snapshot{}, fmt.Errorf("检查重复快照失败: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO tm_ai_source_bodies (content_hash, body_text, sentences_json, created_at)
	  VALUES (?, ?, ?, UTC_TIMESTAMP(3))`, snapshot.Hash, snapshot.Body, string(serializedSentences)); err != nil {
		return sources.Snapshot{}, fmt.Errorf("保存不可变资料正文失败: %w", err)
	}
	var storedBody string
	var storedSentencesJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT body_text, sentences_json FROM tm_ai_source_bodies WHERE content_hash = ?`, snapshot.Hash).
		Scan(&storedBody, &storedSentencesJSON); err != nil {
		return sources.Snapshot{}, fmt.Errorf("复核内容寻址正文失败: %w", err)
	}
	var storedSentences []sources.Sentence
	if err := json.Unmarshal(storedSentencesJSON, &storedSentences); err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析已存句子索引失败: %w", err)
	}
	if storedBody != snapshot.Body || !slices.Equal(storedSentences, snapshot.Sentences) {
		return sources.Snapshot{}, fmt.Errorf("同一内容哈希对应的已存正文不一致")
	}
	const insert = `INSERT INTO tm_ai_source_snapshots
	  (source_id, category_snapshot, url, url_hash, content_hash, body_ref,
	   license_tag_snapshot, license_ref_snapshot, license_expires_snapshot, fetched_at, created_at)
	  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(3))`
	result, err := tx.ExecContext(ctx, insert, snapshot.SourceID, snapshot.SourceCategory, snapshot.URL, urlHash[:], snapshot.Hash, snapshot.Hash,
		snapshot.LicenseTag, snapshot.LicenseRef, nullableTime(snapshot.LicenseExpires), snapshot.FetchedAt.UTC().Format(databaseUTCLayout))
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("写入不可变资料快照失败: %w", err)
	}
	snapshot.ID, err = result.LastInsertId()
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("读取快照编号失败: %w", err)
	}
	return snapshot, nil
}

// SnapshotByID 读取快照正文与偏移索引，供后续抽取流水线按句编号取证。
func (r *AISourceRepo) SnapshotByID(ctx context.Context, id int64) (sources.Snapshot, error) {
	return sourceSnapshotByID(ctx, r.db, id, false)
}

func sourceSnapshotByID(ctx context.Context, db sourceSQL, id int64, share bool) (sources.Snapshot, error) {
	if id <= 0 {
		return sources.Snapshot{}, sources.ErrSourceUnavailable
	}
	const query = `SELECT s.id, s.source_id, s.category_snapshot, s.url, s.content_hash, b.body_text, b.sentences_json,
	    s.license_tag_snapshot, s.license_ref_snapshot,
	    DATE_FORMAT(s.license_expires_snapshot, '` + databaseUTCSelect + `'), DATE_FORMAT(s.fetched_at, '` + databaseUTCSelect + `')
	  FROM tm_ai_source_snapshots s JOIN tm_ai_source_bodies b ON b.content_hash = s.body_ref
	  WHERE s.id = ?`
	selected := query
	if share {
		selected += ` FOR SHARE`
	}
	snapshot, err := scanSnapshot(db.QueryRowContext(ctx, selected, id))
	if errors.Is(err, sql.ErrNoRows) {
		return sources.Snapshot{}, sources.ErrSourceUnavailable
	}
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("读取资料快照失败: %w", err)
	}
	return snapshot, nil
}

// SnapshotForExtraction 仅为当前已启用且授权仍有效的来源返回快照。
// 快照保存时的授权副本必须与当前登记一致，防止来源授权撤回后继续送入模型。
func (r *AISourceRepo) SnapshotForExtraction(ctx context.Context, id int64) (sources.Snapshot, error) {
	snapshot, err := r.SnapshotByID(ctx, id)
	if err != nil {
		return sources.Snapshot{}, err
	}
	source, err := r.SourceByID(ctx, snapshot.SourceID)
	if err != nil {
		return sources.Snapshot{}, err
	}
	var databaseNowRaw string
	if err := r.db.QueryRowContext(ctx, `SELECT DATE_FORMAT(UTC_TIMESTAMP(3), '`+databaseUTCSelect+`')`).Scan(&databaseNowRaw); err != nil {
		return sources.Snapshot{}, fmt.Errorf("读取来源授权复核时点失败: %w", err)
	}
	databaseNow, err := time.Parse(databaseUTCLayout, databaseNowRaw)
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析来源授权复核时点失败: %w", err)
	}
	if !source.HasValidAuthorization(databaseNow.UTC()) || source.Category != snapshot.SourceCategory ||
		source.LicenseTag != snapshot.LicenseTag || source.LicenseRef != snapshot.LicenseRef ||
		!sameOptionalTime(source.LicenseExpires, snapshot.LicenseExpires) {
		return sources.Snapshot{}, sources.ErrUnauthorized
	}
	canonicalURL, err := sources.ValidateURL(source.Domain, snapshot.URL)
	if err != nil || canonicalURL != snapshot.URL {
		return sources.Snapshot{}, sources.ErrUnsafeURL
	}
	return snapshot, nil
}

type rowScanner interface{ Scan(...any) error }

func scanSnapshot(row rowScanner) (sources.Snapshot, error) {
	var snapshot sources.Snapshot
	var rawSentences []byte
	var expires sql.NullString
	var fetchedAtRaw string
	err := row.Scan(&snapshot.ID, &snapshot.SourceID, &snapshot.SourceCategory, &snapshot.URL, &snapshot.Hash, &snapshot.Body, &rawSentences,
		&snapshot.LicenseTag, &snapshot.LicenseRef, &expires, &fetchedAtRaw)
	if err != nil {
		return sources.Snapshot{}, err
	}
	if snapshot.LicenseExpires, err = parseDatabaseUTC(expires); err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析快照授权到期时间失败: %w", err)
	}
	fetchedAt, err := time.Parse(databaseUTCLayout, fetchedAtRaw)
	if err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析快照采集时间失败: %w", err)
	}
	snapshot.FetchedAt = fetchedAt.UTC()
	if err := json.Unmarshal(rawSentences, &snapshot.Sentences); err != nil {
		return sources.Snapshot{}, fmt.Errorf("解析快照句子索引失败: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return sources.Snapshot{}, fmt.Errorf("快照完整性校验失败: %w", err)
	}
	return snapshot, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(databaseUTCLayout)
}

func parseDatabaseUTC(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := time.Parse(databaseUTCLayout, value.String)
	if err != nil {
		return nil, fmt.Errorf("时间格式无效: %w", err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func sameOptionalTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func affectedOrUnavailable(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("读取资料来源更新结果失败: %w", err)
	}
	if count == 0 {
		return sources.ErrSourceUnavailable
	}
	return nil
}
