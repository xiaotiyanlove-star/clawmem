package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/philippgille/chromem-go"
	"github.com/xiaotiyanlove-star/clawmem/config"
)

// EmbeddingProvider 定义获取向量的接口，解耦 embedding 包依赖
type EmbeddingProvider interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, string, error)
}

// VectorStore 封装 chromem-go 向量数据库
type VectorStore struct {
	db         *chromem.DB
	collection *chromem.Collection
	embedder   EmbeddingProvider
	cfg        *config.Config
}

// NewVectorStore 创建并初始化向量存储
func NewVectorStore(cfg *config.Config, embedder EmbeddingProvider) (*VectorStore, error) {
	// 确保目录存在
	if err := os.MkdirAll(cfg.VectorDBPath, 0o755); err != nil {
		return nil, fmt.Errorf("创建向量库目录失败: %w", err)
	}

	// 创建持久化 DB
	db, err := chromem.NewPersistentDB(cfg.VectorDBPath, false)
	if err != nil {
		return nil, fmt.Errorf("创建向量数据库失败: %w", err)
	}

	// 创建自定义 Embedding 函数
	embeddingFunc := func(ctx context.Context, text string) ([]float32, error) {
		vec, _, err := embedder.GetEmbedding(ctx, text)
		return vec, err
	}

	// 获取或创建 Collection
	collection, err := db.GetOrCreateCollection("memories", nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("创建向量集合失败: %w", err)
	}

	return &VectorStore{
		db:         db,
		collection: collection,
		embedder:   embedder,
		cfg:        cfg,
	}, nil
}

// Add 添加一个文档到向量库
func (v *VectorStore) Add(ctx context.Context, id string, content string, metadata map[string]string, embedding []float32) error {
	doc := chromem.Document{
		ID:       id,
		Content:  content,
		Metadata: metadata,
	}
	if len(embedding) > 0 {
		doc.Embedding = embedding
	}
	return v.collection.AddDocument(ctx, doc)
}

// Query 检索最相似的文档
func (v *VectorStore) Query(ctx context.Context, queryText string, topK int, whereFilter map[string]string) ([]chromem.Result, error) {
	if topK <= 0 {
		topK = 5
	}

	// Fix: Ensure topK does not exceed total document count
	count := v.collection.Count()
	if count == 0 {
		return []chromem.Result{}, nil
	}
	if topK > count {
		topK = count
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

// Delete 从向量库中直接删除指定的 ID 列表的向量文档
func (v *VectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return v.collection.Delete(ctx, nil, nil, ids...)
}
