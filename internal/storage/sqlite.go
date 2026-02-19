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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
	CREATE INDEX IF NOT EXISTS idx_memories_session ON memories(user_id, session_id);

	CREATE TABLE IF NOT EXISTS embedding_cache (
		hash TEXT PRIMARY KEY,
		vector TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// GetCachedEmbedding 获取缓存的向量
func (s *SQLiteStore) GetCachedEmbedding(hash string) ([]float32, error) {
	var vectorJSON string
	err := s.db.QueryRow("SELECT vector FROM embedding_cache WHERE hash = ?", hash).Scan(&vectorJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var vector []float32
	if err := json.Unmarshal([]byte(vectorJSON), &vector); err != nil {
		return nil, err
	}
	return vector, nil
}

// SetCachedEmbedding 设置缓存向量
func (s *SQLiteStore) SetCachedEmbedding(hash string, vector []float32) error {
	vectorJSON, err := json.Marshal(vector)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO embedding_cache (hash, vector)
		VALUES (?, ?)
	`, hash, string(vectorJSON))
	return err
}

// Insert 插入一条记忆
func (s *SQLiteStore) Insert(m *model.Memory) error {
	tagsJSON, _ := json.Marshal(m.Tags)
	_, err := s.db.Exec(
		`INSERT INTO memories (id, user_id, session_id, content, summary, source, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.UserID, m.SessionID, m.Content, m.Summary, m.Source, string(tagsJSON), m.CreatedAt, m.UpdatedAt,
	)
	return err
}

// GetByID 根据 ID 获取记忆
func (s *SQLiteStore) GetByID(id string) (*model.Memory, error) {
	row := s.db.QueryRow(`SELECT id, user_id, session_id, content, summary, source, tags, created_at, updated_at FROM memories WHERE id = ?`, id)
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
		`SELECT id, user_id, session_id, content, summary, source, tags, created_at, updated_at
		FROM memories WHERE id IN (%s)`,
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
	var tagsJSON string
	var createdAt, updatedAt string
	err := row.Scan(&m.ID, &m.UserID, &m.SessionID, &m.Content, &m.Summary, &m.Source, &tagsJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &m, nil
}

// scanMemoryFromRows 从多行查询结果扫描 Memory
func scanMemoryFromRows(rows *sql.Rows) (*model.Memory, error) {
	var m model.Memory
	var tagsJSON string
	var createdAt, updatedAt string
	err := rows.Scan(&m.ID, &m.UserID, &m.SessionID, &m.Content, &m.Summary, &m.Source, &tagsJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &m, nil
}
