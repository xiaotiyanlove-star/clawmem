package model

import "time"

// Memory 记忆条目数据结构
type Memory struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id,omitempty"`
	Content   string    `json:"content"`
	Summary   string    `json:"summary,omitempty"`
	Source    string    `json:"source,omitempty"` // 来源标识，如 "chat", "document"
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	UserID    string `json:"user_id" form:"user_id" binding:"required"`
	Query     string `json:"query" form:"query" binding:"required"`
	TopK      int    `json:"top_k" form:"top_k"`
	SessionID string `json:"session_id" form:"session_id,omitempty"`
}

// SearchResult 检索结果
type SearchResult struct {
	Memory     Memory  `json:"memory"`
	Score      float32 `json:"score"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status      string `json:"status"`
	MemoryCount int64  `json:"memory_count"`
	Version     string `json:"version"`
}
