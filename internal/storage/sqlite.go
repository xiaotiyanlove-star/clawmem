package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xiaotiyanlove-star/clawmem/internal/model"
	_ "modernc.org/sqlite"
)

// SQLiteStore 封装 SQLite 数据库操作
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore 创建并初始化 SQLite 存储
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return store, nil
}

// migrate 执行数据库建表
func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL,
		session_id TEXT DEFAULT '',
		content    TEXT NOT NULL,
		summary    TEXT DEFAULT '',
		source     TEXT DEFAULT '',
		tags       TEXT DEFAULT '[]',
		status     TEXT DEFAULT 'active',
		embed_provider TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
	CREATE INDEX IF NOT EXISTS idx_memories_session ON memories(user_id, session_id);
	CREATE INDEX IF NOT EXISTS idx_memories_status ON memories(status);
	CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);

	CREATE TABLE IF NOT EXISTS embedding_cache (
		hash TEXT PRIMARY KEY,
		vector TEXT,
		provider TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS dream_log (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		input_count INTEGER DEFAULT 0,
		output_count INTEGER DEFAULT 0,
		status     TEXT DEFAULT 'running',
		error_msg  TEXT DEFAULT ''
	);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// 兼容旧数据库：尝试添加 status 列（已存在则忽略）
	s.db.Exec("ALTER TABLE memories ADD COLUMN status TEXT DEFAULT 'active'")
	// 兼容旧数据库：尝试添加 embed_provider 列（已存在则忽略）
	s.db.Exec("ALTER TABLE memories ADD COLUMN embed_provider TEXT DEFAULT ''")
	s.db.Exec("ALTER TABLE memories ADD COLUMN deleted_at DATETIME")
	// 兼容旧数据库：尝试添加 provider 列（已存在则忽略）
	s.db.Exec("ALTER TABLE embedding_cache ADD COLUMN provider TEXT DEFAULT ''")
	return nil
}

// GetCachedEmbedding 获取缓存的向量及来源提供商
func (s *SQLiteStore) GetCachedEmbedding(hash string) ([]float32, string, error) {
	var vectorJSON, provider string
	err := s.db.QueryRow("SELECT vector, provider FROM embedding_cache WHERE hash = ?", hash).Scan(&vectorJSON, &provider)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}

	var vector []float32
	if err := json.Unmarshal([]byte(vectorJSON), &vector); err != nil {
		return nil, "", err
	}
	return vector, provider, nil
}

// SetCachedEmbedding 设置缓存向量及来源提供商
func (s *SQLiteStore) SetCachedEmbedding(hash string, vector []float32, provider string) error {
	vectorJSON, err := json.Marshal(vector)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO embedding_cache (hash, vector, provider)
		VALUES (?, ?, ?)
		ON CONFLICT(hash) DO UPDATE SET vector=excluded.vector, provider=excluded.provider
	`, hash, string(vectorJSON), provider)
	return err
}

// Insert 插入一条记忆
func (s *SQLiteStore) Insert(m *model.Memory) error {
	tagsJSON, _ := json.Marshal(m.Tags)
	status := m.Status
	if status == "" {
		status = model.StatusActive
	}
	_, err := s.db.Exec(
		`INSERT INTO memories (id, user_id, session_id, content, summary, source, tags, status, embed_provider, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.UserID, m.SessionID, m.Content, m.Summary, m.Source, string(tagsJSON), status, m.EmbedProvider, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

// GetByID 根据 ID 获取记忆，默认排除已删除
func (s *SQLiteStore) GetByID(id string) (*model.Memory, error) {
	row := s.db.QueryRow(`SELECT id, user_id, session_id, content, summary, source, tags, status, embed_provider, created_at, updated_at, deleted_at FROM memories WHERE id = ? AND deleted_at IS NULL`, id)
	return scanMemory(row)
}

// GetByIDWithDeleted 根据 ID 获取记忆，包含已删除
func (s *SQLiteStore) GetByIDWithDeleted(id string) (*model.Memory, error) {
	row := s.db.QueryRow(`SELECT id, user_id, session_id, content, summary, source, tags, status, embed_provider, created_at, updated_at, deleted_at FROM memories WHERE id = ?`, id)
	return scanMemory(row)
}

// GetByIDs 根据 ID 列表批量获取记忆
func (s *SQLiteStore) GetByIDs(ids []string) ([]*model.Memory, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT id, user_id, session_id, content, summary, source, tags, status, embed_provider, created_at, updated_at, deleted_at
		FROM memories WHERE id IN (%s) AND deleted_at IS NULL`,
		strings.Join(placeholders, ","),
	)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*model.Memory
	for rows.Next() {
		m, err := scanMemoryFromRows(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// GetRecentActive 获取指定时间之后的活跃记忆（用于 Dream 整合）
func (s *SQLiteStore) GetRecentActive(since time.Time, limit int) ([]*model.Memory, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, session_id, content, summary, source, tags, status, embed_provider, created_at, updated_at, deleted_at
		FROM memories
		WHERE status = ? AND created_at >= ? AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT ?`,
		model.StatusActive, since.UTC().Format(time.RFC3339), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*model.Memory
	for rows.Next() {
		m, err := scanMemoryFromRows(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// MarkConsolidated 将指定 ID 的记忆标记为已整合
func (s *SQLiteStore) MarkConsolidated(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = model.StatusConsolidated
	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}
	query := fmt.Sprintf(
		`UPDATE memories SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	_, err := s.db.Exec(query, args...)
	return err
}

// LogDreamStart 记录 Dream 任务开始
func (s *SQLiteStore) LogDreamStart(startedAt time.Time, inputCount int) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO dream_log (started_at, input_count, status) VALUES (?, ?, 'running')`,
		startedAt.UTC().Format(time.RFC3339), inputCount,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// LogDreamFinish 记录 Dream 任务完成
func (s *SQLiteStore) LogDreamFinish(logID int64, outputCount int, errMsg string) error {
	status := "success"
	if errMsg != "" {
		status = "failed"
	}
	_, err := s.db.Exec(
		`UPDATE dream_log SET finished_at = CURRENT_TIMESTAMP, output_count = ?, status = ?, error_msg = ? WHERE id = ?`,
		outputCount, status, errMsg, logID,
	)
	return err
}

// Count 统计记忆总数
func (s *SQLiteStore) Count() (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count)
	return count, err
}

// Close 关闭数据库连接
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// scanMemory 从单行查询结果扫描 Memory
func scanMemory(row *sql.Row) (*model.Memory, error) {
	var m model.Memory
	var tagsJSON, status, provider string
	var createdAt, updatedAt string
	var deletedAt sql.NullString
	err := row.Scan(&m.ID, &m.UserID, &m.SessionID, &m.Content, &m.Summary, &m.Source, &tagsJSON, &status, &provider, &createdAt, &updatedAt, &deletedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	m.Status = status
	m.EmbedProvider = provider
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if deletedAt.Valid && deletedAt.String != "" {
		t, err := time.Parse(time.RFC3339, deletedAt.String)
		if err == nil {
			m.DeletedAt = &t
		}
	}
	return &m, nil
}

// scanMemoryFromRows 从多行查询结果扫描 Memory
func scanMemoryFromRows(rows *sql.Rows) (*model.Memory, error) {
	var m model.Memory
	var tagsJSON, status, provider string
	var createdAt, updatedAt string
	var deletedAt sql.NullString
	err := rows.Scan(&m.ID, &m.UserID, &m.SessionID, &m.Content, &m.Summary, &m.Source, &tagsJSON, &status, &provider, &createdAt, &updatedAt, &deletedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	m.Status = status
	m.EmbedProvider = provider
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if deletedAt.Valid && deletedAt.String != "" {
		t, err := time.Parse(time.RFC3339, deletedAt.String)
		if err == nil {
			m.DeletedAt = &t
		}
	}
	return &m, nil
}

// GetLocalMemories 获取需要被修复为云端推理的本地记忆
func (s *SQLiteStore) GetLocalMemories(limit int) ([]*model.Memory, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, session_id, content, summary, source, tags, status, embed_provider, created_at, updated_at, deleted_at
		FROM memories
		WHERE status = ? AND embed_provider = ? AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT ?`,
		model.StatusActive, "local", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*model.Memory
	for rows.Next() {
		m, err := scanMemoryFromRows(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// UpdateMemoryProvider 更新单条记忆的 EmbedProvider
func (s *SQLiteStore) UpdateMemoryProvider(id string, newProvider string) error {
	_, err := s.db.Exec(`UPDATE memories SET embed_provider = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newProvider, id)
	return err
}

// SoftDeleteByID 软删除单条记忆
func (s *SQLiteStore) SoftDeleteByID(id string) error {
	_, err := s.db.Exec(`UPDATE memories SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// SoftDeleteByIDs 批量软删除多条记忆
func (s *SQLiteStore) SoftDeleteByIDs(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(
		`UPDATE memories SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	_, err := s.db.Exec(query, args...)
	return err
}

// UpdateMemRecord 全量更新记忆记录（主要用于覆盖逻辑）
// 支持可选地重置 deleted_at（复活逻辑）
func (s *SQLiteStore) UpdateMemRecord(m *model.Memory, restore bool) error {
	tagsJSON, _ := json.Marshal(m.Tags)
	status := m.Status
	if status == "" {
		status = model.StatusActive
	}

	query := `UPDATE memories SET 
		content = ?, summary = ?, source = ?, tags = ?, status = ?, embed_provider = ?, updated_at = CURRENT_TIMESTAMP`

	if restore {
		query += `, deleted_at = NULL`
	}

	query += ` WHERE id = ?`

	_, err := s.db.Exec(query,
		m.Content, m.Summary, m.Source, string(tagsJSON), status, m.EmbedProvider, m.ID,
	)
	return err
}
