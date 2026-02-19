package core

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/xiaotiyanlove-star/clawmem/config"
	"github.com/xiaotiyanlove-star/clawmem/internal/llm"
	"github.com/xiaotiyanlove-star/clawmem/internal/model"
	"github.com/xiaotiyanlove-star/clawmem/internal/storage"
)

// MemoryService 核心记忆服务
type MemoryService struct {
	cfg         *config.Config
	sqlStore    *storage.SQLiteStore
	vectorStore *storage.VectorStore
	llmClient   *llm.Client
}

// NewMemoryService 创建记忆服务实例
func NewMemoryService(cfg *config.Config, sqlStore *storage.SQLiteStore, vectorStore *storage.VectorStore, llmClient *llm.Client) *MemoryService {
	return &MemoryService{
		cfg:         cfg,
		sqlStore:    sqlStore,
		vectorStore: vectorStore,
		llmClient:   llmClient,
	}
}

// AddMemory 添加一条新记忆
// 流程: 接收文本 -> (可选)LLM提取摘要 -> 存入SQLite(元数据) + Chromem(向量)
func (s *MemoryService) AddMemory(ctx context.Context, req *model.AddMemoryRequest) (*model.Memory, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	// 使用 LLM 提取摘要（如果内容较长且未禁用）
	summary := ""
	if !s.cfg.DisableLLMSummary && len(req.Content) > 200 {
		var err error
		summary, err = s.llmClient.Summarize(ctx, req.Content)
		if err != nil {
			// 摘要失败不阻塞存储，仅记录日志
			log.Printf("[WARN] 记忆摘要提取失败: %v", err)
		}
	}

	// 构建记忆对象
	mem := &model.Memory{
		ID:        id,
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Content:   req.Content,
		Summary:   summary,
		Source:    req.Source,
		Tags:      req.Tags,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 存入 SQLite
	if err := s.sqlStore.Insert(mem); err != nil {
		return nil, fmt.Errorf("存储记忆失败: %w", err)
	}

	// 存入向量库（使用摘要或原文作为 embedding 内容）
	embeddingContent := mem.Content
	if summary != "" {
		embeddingContent = summary
	}

	metadata := map[string]string{
		"user_id":    mem.UserID,
		"session_id": mem.SessionID,
		"source":     mem.Source,
	}

	if err := s.vectorStore.Add(ctx, id, embeddingContent, metadata); err != nil {
		// 向量存储失败不阻塞，元数据已入库
		log.Printf("[WARN] 向量存储失败: %v", err)
	}

	return mem, nil
}

// SearchMemory 检索记忆
// 流程: 接收Query -> Chromem向量检索 -> SQLite补充完整信息 -> 返回结果
func (s *MemoryService) SearchMemory(ctx context.Context, req *model.SearchMemoryRequest) ([]model.SearchResult, error) {
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}

	// 构建过滤条件
	whereFilter := map[string]string{
		"user_id": req.UserID,
	}
	if req.SessionID != "" {
		whereFilter["session_id"] = req.SessionID
	}

	// 向量检索
	vResults, err := s.vectorStore.Query(ctx, req.Query, topK, whereFilter)
	if err != nil {
		return nil, fmt.Errorf("记忆检索失败: %w", err)
	}

	if len(vResults) == 0 {
		return []model.SearchResult{}, nil
	}

	// 提取 ID 列表
	ids := make([]string, len(vResults))
	scoreMap := make(map[string]float32)
	for i, r := range vResults {
		ids[i] = r.ID
		scoreMap[r.ID] = r.Similarity
	}

	// 从 SQLite 获取完整信息
	memories, err := s.sqlStore.GetByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("获取记忆详情失败: %w", err)
	}

	// 组装结果（保持向量检索排序）
	results := make([]model.SearchResult, 0, len(memories))
	for _, id := range ids {
		for _, m := range memories {
			if m.ID == id {
				results = append(results, model.SearchResult{
					Memory: *m,
					Score:  scoreMap[id],
				})
				break
			}
		}
	}

	return results, nil
}

// GetMemoryCount 获取记忆总数
func (s *MemoryService) GetMemoryCount() (int64, error) {
	return s.sqlStore.Count()
}
