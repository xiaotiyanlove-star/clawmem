package model

import "time"

// 记忆状态常量
const (
	StatusActive       = "active"       // 正常活跃记忆
	StatusConsolidated = "consolidated" // 已被 Dream 整合归档
	StatusDream        = "dream"        // Dream 生成的精华记忆
)

// Memory 记忆条目数据结构
type Memory struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	SessionID     string     `json:"session_id,omitempty"`
	Content       string     `json:"content"`
	Summary       string     `json:"summary,omitempty"`
	Source        string     `json:"source,omitempty"` // 来源标识，如 "chat", "document", "dream"
	Tags          []string   `json:"tags,omitempty"`
	Status        string     `json:"status,omitempty"`         // active, consolidated, dream
	EmbedProvider string     `json:"embed_provider,omitempty"` // 生成此记忆向量的具体模型/服务商 (e.g. cloudflare, local)
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// AddMemoryRequest 添加记忆请求
type AddMemoryRequest struct {
	UserID    string   `json:"user_id" binding:"required"`
	SessionID string   `json:"session_id,omitempty"`
	Content   string   `json:"content" binding:"required"`
	Source    string   `json:"source,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// SearchMemoryRequest 检索记忆请求
type SearchMemoryRequest struct {
	UserID         string `json:"user_id" form:"user_id" binding:"required"`
	Query          string `json:"query" form:"query" binding:"required"`
	TopK           int    `json:"top_k,string" form:"top_k,default=5"` // Add default tag and allow string unmarshaling if passed as JSON
	SessionID      string `json:"session_id" form:"session_id,omitempty"`
	IncludeDeleted bool   `json:"include_deleted" form:"include_deleted,omitempty"`
}

// SearchResult 检索结果
type SearchResult struct {
	Memory Memory  `json:"memory"`
	Score  float32 `json:"score"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status      string `json:"status"`
	MemoryCount int64  `json:"memory_count"`
	Version     string `json:"version"`
}

// DeleteByQueryRequest 通过语义批量删除记忆
type DeleteByQueryRequest struct {
	UserID    string  `json:"user_id" binding:"required"`
	Query     string  `json:"query" binding:"required"`
	Threshold float32 `json:"threshold,omitempty"` // 默认 0.85
}

// SetMemoryRequest 智能覆盖记忆请求
type SetMemoryRequest struct {
	UserID         string            `json:"user_id" binding:"required"`
	ID             string            `json:"id,omitempty"` // 可选：若提供则更新该 ID，否则新建
	Content        string            `json:"content" binding:"required"`
	Metadata       map[string]string `json:"metadata,omitempty"` // 当前只用到 source 和 tag
	Source         string            `json:"source,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	MatchQuery     string            `json:"match_query,omitempty"`     // 可选：匹配相似内容
	MatchThreshold float32           `json:"match_threshold,omitempty"` // 默认 0.85
}

// UpdateMemoryRequest 简单覆盖记忆请求
type UpdateMemoryRequest struct {
	Content  string            `json:"content" binding:"required"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Source   string            `json:"source,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
}
