package embedding

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

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

	// Provider 状态 (true = 可用, false = 不可用/降级)
	statusMu     sync.RWMutex
	providerDown map[string]bool

	// 并发控制
	cloudSem chan struct{} // 云端高并发 (e.g., 20)
	localSem chan struct{} // 本地低并发 (e.g., 2)
}

func NewManager(cfg *config.Config, cacheStore *storage.SQLiteStore) *Manager {
	m := &Manager{
		cfg:          cfg,
		cacheStore:   cacheStore,
		local:        NewLocalEmbedder(),
		cloudflare:   NewCloudflareEmbedder(cfg.CFAccountID, cfg.CFAPIToken),
		openai:       NewOpenAIEmbedder(cfg),
		providerDown: make(map[string]bool),
		cloudSem:     make(chan struct{}, 20),
		localSem:     make(chan struct{}, 2),
	}

	// 异步启动自检
	go m.healthCheck()

	return m
}

// healthCheck 启动时检查已配置的 Provider 是否可用
func (m *Manager) healthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("[INFO] Starting Embedding Provider Health Check...")

	check := func(e Embedder, isLocal bool) {
		if !m.isConfigured(e) {
			return
		}
		// 尝试 embed 一个简单的单词
		// 注意: 本地模型不需要 check，因为是 lazy load 且始终视为可用（除非 load 失败）
		if isLocal {
			return
		}

		// 使用信号量防止 startup 时的滥用
		m.cloudSem <- struct{}{}
		_, err := e.Embed(ctx, "ping")
		<-m.cloudSem

		if err != nil {
			log.Printf("[WARN] Health Check Failed for %s: %v. Marking as DOWN.", e.Name(), err)
			m.markDown(e.Name())
		} else {
			log.Printf("[INFO] Health Check Passed: %s", e.Name())
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); check(m.openai, false) }()
	go func() { defer wg.Done(); check(m.cloudflare, false) }()
	wg.Wait()
}

func (m *Manager) markDown(name string) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	m.providerDown[name] = true
}

func (m *Manager) isDown(name string) bool {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	return m.providerDown[name]
}

// GetEmbedding 获取文本向量 (单条，兼容旧接口)
func (m *Manager) GetEmbedding(ctx context.Context, text string) ([]float32, string, error) {
	vecs, providers, err := m.GetEmbeddingBatch(ctx, []string{text})
	if err != nil {
		return nil, "", err
	}
	if len(vecs) > 0 {
		return vecs[0], providers[0], nil
	}
	return nil, "", fmt.Errorf("empty result")
}

// GetEmbeddingBatch 批量获取向量 (含缓存 + 策略降级 + 差量计算)
func (m *Manager) GetEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, []string, error) {
	finalResults := make([][]float32, len(texts))
	finalProviders := make([]string, len(texts))
	hashes := make([]string, len(texts))

	// 记录需要计算的文本索引和内容
	var missingIndices []int
	var missingTexts []string

	// 1. 检查缓存
	for i, text := range texts {
		h := md5Hash(text)
		hashes[i] = h
		if vec, provider, err := m.cacheStore.GetCachedEmbedding(h); err == nil && vec != nil {
			finalResults[i] = vec
			finalProviders[i] = provider
		} else {
			missingIndices = append(missingIndices, i)
			missingTexts = append(missingTexts, text)
		}
	}

	// 如果全部命中缓存，直接返回（Zero API Call）
	if len(missingTexts) == 0 {
		return finalResults, finalProviders, nil
	}

	log.Printf("[INFO] Cache Hit: %d/%d. Fetching %d missing embeddings...", len(texts)-len(missingTexts), len(texts), len(missingTexts))

	// 2. 仅对缺失部分执行 Embedding
	var computedVecs [][]float32
	var computedProvider string
	var err error

	strategy := m.cfg.EmbeddingStrategy
	switch strategy {
	case "accuracy_first":
		computedVecs, computedProvider, err = m.tryChainBatch(ctx, missingTexts, m.openai, m.cloudflare, m.local)
	case "local_only":
		computedVecs, computedProvider, err = m.tryChainBatch(ctx, missingTexts, m.local)
	case "cloud_first":
		fallthrough
	default:
		computedVecs, computedProvider, err = m.tryChainBatch(ctx, missingTexts, m.cloudflare, m.local)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("all embedding strategies failed for missing items: %w", err)
	}

	if len(computedVecs) != len(missingTexts) {
		return nil, nil, fmt.Errorf("embedding count mismatch: sent %d, got %d", len(missingTexts), len(computedVecs))
	}

	// 3. 合并结果并写入缓存
	for i, idx := range missingIndices {
		vec := computedVecs[i]
		finalResults[idx] = vec // 填回原位
		finalProviders[idx] = computedProvider

		// 写入缓存
		if err := m.cacheStore.SetCachedEmbedding(hashes[idx], vec, computedProvider); err != nil {
			log.Printf("[WARN] Failed to cache embedding: %v", err)
		}
	}

	return finalResults, finalProviders, nil
}

// ForceCloudEmbeddingBatch 强制使用云端执行大批量 Embedding（绕过缓存并强制覆盖）
func (m *Manager) ForceCloudEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, string, error) {
	// 验证 Cloud 是否可用 (仅尝试 Cloudflare 和 OpenAI)
	vecs, provider, err := m.tryChainBatch(ctx, texts, m.cloudflare, m.openai)
	if err != nil {
		return nil, "", fmt.Errorf("cloud embed failed during heal: %w", err)
	}

	// 强制重写 Cache
	for i, text := range texts {
		h := md5Hash(text)
		if err := m.cacheStore.SetCachedEmbedding(h, vecs[i], provider); err != nil {
			log.Printf("[WARN] Failed to overwrite cache during heal: %v", err)
		}
	}

	return vecs, provider, nil
}

// tryChainBatch 尝试执行链 (Batch)
func (m *Manager) tryChainBatch(ctx context.Context, texts []string, embedders ...Embedder) ([][]float32, string, error) {
	var lastErr error
	for _, e := range embedders {
		if !m.isConfigured(e) || m.isDown(e.Name()) {
			continue
		}

		// 获取合适的信号量
		sem := m.cloudSem
		if _, ok := e.(*LocalEmbedder); ok {
			sem = m.localSem
		}

		sem <- struct{}{}
		vecs, err := e.EmbedBatch(ctx, texts)
		<-sem

		if err == nil {
			log.Printf("[INFO] Embedded batch (%d items) using %s", len(texts), e.Name())
			return vecs, e.Name(), nil
		}

		log.Printf("[WARN] Embedder %s failed: %v", e.Name(), err)
		lastErr = err
	}
	return nil, "", lastErr
}

// isConfigured 检查 Embedder 是否配置可用
func (m *Manager) isConfigured(e Embedder) bool {
	switch v := e.(type) {
	case *OpenAIEmbedder:
		return v.BaseURL != "" && v.APIKey != ""
	case *CloudflareEmbedder:
		return v.AccountID != "" && v.APIToken != ""
	case *LocalEmbedder:
		return true
	default:
		return false
	}
}

func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
