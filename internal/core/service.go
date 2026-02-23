package core

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/xiaotiyanlove-star/clawmem/config"
	"github.com/xiaotiyanlove-star/clawmem/internal/embedding"
	"github.com/xiaotiyanlove-star/clawmem/internal/llm"
	"github.com/xiaotiyanlove-star/clawmem/internal/model"
	"github.com/xiaotiyanlove-star/clawmem/internal/storage"
)

// MemoryService 核心记忆服务
type MemoryService struct {
	cfg          *config.Config
	sqlStore     *storage.SQLiteStore
	vectorStore  *storage.VectorStore
	llmClient    *llm.Client
	embedManager *embedding.Manager
}

// NewMemoryService 创建记忆服务实例
func NewMemoryService(cfg *config.Config, sqlStore *storage.SQLiteStore, vectorStore *storage.VectorStore, llmClient *llm.Client, embedManager *embedding.Manager) *MemoryService {
	return &MemoryService{
		cfg:          cfg,
		sqlStore:     sqlStore,
		vectorStore:  vectorStore,
		llmClient:    llmClient,
		embedManager: embedManager,
	}
}

// AddMemory 添加一条新记忆
// 流程: 接收文本 -> (可选)LLM提取摘要 -> 存入SQLite(元数据) + embeddingManager获取向量 -> Chromem(向量)
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

	// 定位 embedding 的内容
	embeddingContent := req.Content
	if summary != "" {
		embeddingContent = summary
	}

	// 显式调用 Embedding 引擎，获取生成的向量和来源提供商
	vec, provider, err := s.embedManager.GetEmbedding(ctx, embeddingContent)
	if err != nil {
		return nil, fmt.Errorf("生成记忆向量失败: %w", err)
	}

	// 构建记忆对象
	mem := &model.Memory{
		ID:            id,
		UserID:        req.UserID,
		SessionID:     req.SessionID,
		Content:       req.Content,
		Summary:       summary,
		Source:        req.Source,
		Tags:          req.Tags,
		Status:        model.StatusActive,
		EmbedProvider: provider, // 记录是谁生成的（"cloudflare", "local", 等）
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 存入 SQLite
	if err := s.sqlStore.Insert(mem); err != nil {
		return nil, fmt.Errorf("存储记忆存入 SQLite 失败: %w", err)
	}

	metadata := map[string]string{
		"user_id":    mem.UserID,
		"session_id": mem.SessionID,
		"source":     mem.Source,
	}

	// 存入向量库（显式传入此前生成的向量，跳过底层二次生成）
	if err := s.vectorStore.Add(ctx, id, embeddingContent, metadata, vec); err != nil {
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

// DeleteMemoryByID 软删除单条记忆
func (s *MemoryService) DeleteMemoryByID(ctx context.Context, id string) error {
	if err := s.sqlStore.SoftDeleteByID(id); err != nil {
		return fmt.Errorf("数据库软删失败: %w", err)
	}
	if err := s.vectorStore.Delete(ctx, []string{id}); err != nil {
		log.Printf("[WARN] 向量清理失败: %v", err)
	}
	return nil
}

// DeleteMemoriesByQuery 根据语义查询软删除关联记忆
func (s *MemoryService) DeleteMemoriesByQuery(ctx context.Context, req *model.DeleteByQueryRequest) (int, []string, error) {
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.85
	}

	// 先执行搜索找出高相关的记忆片段
	searchReq := &model.SearchMemoryRequest{
		UserID: req.UserID,
		Query:  req.Query,
		TopK:   50, // 设定一个合理的批量上限
	}

	results, err := s.SearchMemory(ctx, searchReq)
	if err != nil {
		return 0, nil, err
	}

	var idsToDelete []string
	for _, res := range results {
		if res.Score >= threshold {
			idsToDelete = append(idsToDelete, res.Memory.ID)
		}
	}

	if len(idsToDelete) == 0 {
		return 0, nil, nil
	}

	if err := s.sqlStore.SoftDeleteByIDs(idsToDelete); err != nil {
		return 0, nil, fmt.Errorf("批量软删失败: %w", err)
	}

	if err := s.vectorStore.Delete(ctx, idsToDelete); err != nil {
		log.Printf("[WARN] 批量向量清理失败: %v", err)
	}

	return len(idsToDelete), idsToDelete, nil
}

// SetMemory 智能覆盖或新增内容
func (s *MemoryService) SetMemory(ctx context.Context, req *model.SetMemoryRequest) (*model.Memory, error) {
	matchQuery := req.MatchQuery
	if matchQuery == "" {
		matchQuery = req.Content
	}

	// 1. 根据 query 进行相近检索并删除以避免重复内容并存（乐观删除）
	delReq := &model.DeleteByQueryRequest{
		UserID:    req.UserID,
		Query:     matchQuery,
		Threshold: req.MatchThreshold,
	}
	_, _, delErr := s.DeleteMemoriesByQuery(ctx, delReq)
	if delErr != nil {
		log.Printf("[WARN] SetMemory 前置排重失败: %v", delErr)
	}

	// 2. 判断是否指定 ID 并需要 Update 复活
	if req.ID != "" {
		existing, err := s.sqlStore.GetByIDWithDeleted(req.ID)
		if err == nil && existing != nil && existing.UserID == req.UserID {
			// 如果记录存在并且属于该用户，执行覆写与复活流程

			summary := ""
			if !s.cfg.DisableLLMSummary && len(req.Content) > 200 {
				summary, _ = s.llmClient.Summarize(ctx, req.Content)
			}
			embeddingContent := req.Content
			if summary != "" {
				embeddingContent = summary
			}
			vec, provider, vErr := s.embedManager.GetEmbedding(ctx, embeddingContent)
			if vErr != nil {
				return nil, fmt.Errorf("生成记忆向量失败: %w", vErr)
			}

			existing.Content = req.Content
			existing.Summary = summary
			if req.Source != "" {
				existing.Source = req.Source
			}
			if len(req.Tags) > 0 {
				existing.Tags = req.Tags
			}
			existing.EmbedProvider = provider

			// 修改并重置 deleted_at
			if uErr := s.sqlStore.UpdateMemRecord(existing, true); uErr != nil {
				return nil, fmt.Errorf("复活并全量更新数据失败: %w", uErr)
			}

			// 重新向向量库提交记录（chromem-go 中 add 会覆盖同 ID 内容，但之前已被删除了）
			s.vectorStore.Add(ctx, existing.ID, embeddingContent, map[string]string{
				"user_id":    existing.UserID,
				"session_id": existing.SessionID,
				"source":     existing.Source,
			}, vec)

			return existing, nil
		}
	}

	// 若没传入 ID 或查询失败则退化为完全新建的流程
	addReq := &model.AddMemoryRequest{
		UserID:  req.UserID,
		Content: req.Content,
		Source:  req.Source,
		Tags:    req.Tags,
	}
	return s.AddMemory(ctx, addReq)
}

// UpdateMemory 指定明确存在且活跃的记忆覆盖修改 (简单覆盖)
func (s *MemoryService) UpdateMemory(ctx context.Context, id string, req *model.UpdateMemoryRequest) (*model.Memory, error) {
	// 使用普通 GetByID，这会自动排除已被软删的
	existing, err := s.sqlStore.GetByID(id)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("目标记忆不存在或已被删除: %w", err)
	}

	summary := ""
	if !s.cfg.DisableLLMSummary && len(req.Content) > 200 {
		summary, _ = s.llmClient.Summarize(ctx, req.Content)
	}
	embeddingContent := req.Content
	if summary != "" {
		embeddingContent = summary
	}
	vec, provider, vErr := s.embedManager.GetEmbedding(ctx, embeddingContent)
	if vErr != nil {
		return nil, fmt.Errorf("生成记忆向量失败: %w", vErr)
	}

	existing.Content = req.Content
	existing.Summary = summary
	if req.Source != "" {
		existing.Source = req.Source
	}
	if len(req.Tags) > 0 {
		existing.Tags = req.Tags
	}
	existing.EmbedProvider = provider

	if err := s.sqlStore.UpdateMemRecord(existing, false); err != nil {
		return nil, fmt.Errorf("更新数据库记录失败: %w", err)
	}

	s.vectorStore.Add(ctx, existing.ID, embeddingContent, map[string]string{
		"user_id":    existing.UserID,
		"session_id": existing.SessionID,
		"source":     existing.Source,
	}, vec)

	return existing, nil
}
