package embedding

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/xiaotiyanlove-star/clawmem/config"
	"github.com/xiaotiyanlove-star/clawmem/internal/storage"
)

// Manager 管理 Embedding 的生成、缓存和策略降级
type Manager struct {
	cfg        *config.Config
	cacheStore *storage.SQLiteStore

	// Embedders
	local      *LocalEmbedder
	cloudflare *CloudflareEmbedder
	openai     *OpenAIEmbedder

	// 并发控制 (简单的信号量，防止本地/云端并发过大)
	sem chan struct{}
}

func NewManager(cfg *config.Config, cacheStore *storage.SQLiteStore) *Manager {
	return &Manager{
		cfg:        cfg,
		cacheStore: cacheStore,
		local:      NewLocalEmbedder(), // 总是初始化本地兜底
		cloudflare: NewCloudflareEmbedder(cfg.CFAccountID, cfg.CFAPIToken),
		openai:     NewOpenAIEmbedder(cfg),
		sem:        make(chan struct{}, 5), // 限制最大 5 并发
	}
}

// GetEmbedding 获取文本向量 (含缓存 + 策略降级)
func (m *Manager) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// 1. 检查缓存
	hash := md5Hash(text)
	if cached, err := m.cacheStore.GetCachedEmbedding(hash); err == nil && cached != nil {
		return cached, nil
	}

	// 2. 并发控制
	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	var vec []float32
	var err error

	// 3. 根据策略选择执行链
	strategy := m.cfg.EmbeddingStrategy
	switch strategy {
	case "accuracy_first":
		vec, err = m.tryChain(ctx, text, m.openai, m.cloudflare, m.local)
	case "local_only":
		vec, err = m.tryChain(ctx, text, m.local)
	case "cloud_first":
		fallthrough
	default: // 默认 cloud_first
		vec, err = m.tryChain(ctx, text, m.cloudflare, m.local)
	}

	if err != nil {
		return nil, fmt.Errorf("all embedding strategies failed: %w", err)
	}

	// 4. 写入缓存
	if err := m.cacheStore.SetCachedEmbedding(hash, vec); err != nil {
		log.Printf("[WARN] Failed to cache embedding: %v", err)
	}

	return vec, nil
}

// tryChain 尝试按顺序调用 Embedder，成功即返回
func (m *Manager) tryChain(ctx context.Context, text string, embedders ...Embedder) ([]float32, error) {
	var lastErr error
	for _, e := range embedders {
		// 跳过未配置的 Embedder
		if !m.isConfigured(e) {
			continue
		}

		vec, err := e.Embed(ctx, text)
		if err == nil {
			log.Printf("[INFO] Embedded using %s", e.Name())
			return vec, nil
		}

		log.Printf("[WARN] Embedder %s failed: %v", e.Name(), err)
		lastErr = err
	}
	return nil, lastErr
}

// isConfigured 检查 Embedder 是否配置可用
func (m *Manager) isConfigured(e Embedder) bool {
	switch v := e.(type) {
	case *OpenAIEmbedder:
		return v.BaseURL != "" && v.APIKey != ""
	case *CloudflareEmbedder:
		return v.AccountID != "" && v.APIToken != ""
	case *LocalEmbedder:
		return true // 本地总是可用
	default:
		return false
	}
}

func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
