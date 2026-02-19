package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/philippgille/chromem-go"
	"github.com/xiaotiyanlove-star/clawmem/config"
	"github.com/xiaotiyanlove-star/clawmem/internal/llm"
)

// VectorStore 封装 chromem-go 向量数据库
type VectorStore struct {
	db         *chromem.DB
	collection *chromem.Collection
	llmClient  *llm.Client
	cfg        *config.Config
}

// NewVectorStore 创建并初始化向量存储
func NewVectorStore(cfg *config.Config, llmClient *llm.Client) (*VectorStore, error) {
	// 确保目录存在
	if err := os.MkdirAll(cfg.VectorDBPath, 0o755); err != nil {
		return nil, fmt.Errorf("创建向量库目录失败: %w", err)
	}

	// 创建持久化 DB
	db, err := chromem.NewPersistentDB(cfg.VectorDBPath, false)
	if err != nil {
		return nil, fmt.Errorf("创建向量数据库失败: %w", err)
	}

	// 创建自定义 Embedding 函数，对接外部 API
	embeddingFunc := func(ctx context.Context, text string) ([]float32, error) {
		return llmClient.Embed(ctx, text)
	}

	// 获取或创建 Collection
	collection, err := db.GetOrCreateCollection("memories", nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("创建向量集合失败: %w", err)
	}

	return &VectorStore{
		db:         db,
		collection: collection,
		llmClient:  llmClient,
		cfg:        cfg,
	}, nil
}

// Add 添加一个文档到向量库
func (v *VectorStore) Add(ctx context.Context, id string, content string, metadata map[string]string) error {
	doc := chromem.Document{
		ID:       id,
		Content:  content,
		Metadata: metadata,
	}
	return v.collection.AddDocument(ctx, doc)
}

// Query 检索最相似的文档
func (v *VectorStore) Query(ctx context.Context, queryText string, topK int, whereFilter map[string]string) ([]chromem.Result, error) {
	if topK <= 0 {
		topK = 5
	}

	var where map[string]string
	if len(whereFilter) > 0 {
		where = whereFilter
	}

	results, err := v.collection.Query(ctx, queryText, topK, where, nil)
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}

	return results, nil
}

// Count 返回向量库中的文档数量
func (v *VectorStore) Count() int {
	return v.collection.Count()
}
