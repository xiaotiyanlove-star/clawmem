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
	// 使用 sync.Map 或 atomic 更好，这里简单用 mutex 保护 map
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
func (m *Manager) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	vecs, err := m.GetEmbeddingBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) > 0 {
		return vecs[0], nil
	}
	return nil, fmt.Errorf("empty result")
}

// GetEmbeddingBatch 批量获取向量 (含缓存 + 策略降级)
func (m *Manager) GetEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// 1. 预检查缓存
	// 复杂的批量缓存逻辑：部分命中缓存，部分未命中。
	// 为简化实现，这里先只查缓存，如果全部命中则直接返回；否则（只要有一个未命中）就全量调 API。
	// TODO: 实现精细的“部分缓存命中”合并逻辑（Diff）
	allCached := true
	cachedResults := make([][]float32, len(texts))
	hashes := make([]string, len(texts))

	for i, text := range texts {
		h := md5Hash(text)
		hashes[i] = h
		if vec, err := m.cacheStore.GetCachedEmbedding(h); err == nil && vec != nil {
			cachedResults[i] = vec
		} else {
			allCached = false
			// 只要有一个没命中，简单起见我们先不 break，继续查完，看是否碰巧全命中
		}
	}

	if allCached {
		return cachedResults, nil
	}

	// 2. 执行 Embedding 策略 (Batch)
	var vecs [][]float32
	var err error

	strategy := m.cfg.EmbeddingStrategy
	switch strategy {
	case "accuracy_first":
		vecs, err = m.tryChainBatch(ctx, texts, m.openai, m.cloudflare, m.local)
	case "local_only":
		vecs, err = m.tryChainBatch(ctx, texts, m.local)
	case "cloud_first":
		fallthrough
	default:
		vecs, err = m.tryChainBatch(ctx, texts, m.cloudflare, m.local)
	}

	if err != nil {
		return nil, fmt.Errorf("all embedding strategies failed: %w", err)
	}

	// 3. 写入缓存 (异步或同步)
	// 这里用同步防止丢失，性能影响可接受
	for i, vec := range vecs {
		if cachedResults[i] == nil { // 只写入未命中的
			if err := m.cacheStore.SetCachedEmbedding(hashes[i], vec); err != nil {
				log.Printf("[WARN] Failed to cache embedding: %v", err)
			}
		}
	}

	return vecs, nil
}

// tryChainBatch 尝试执行链 (Batch)
func (m *Manager) tryChainBatch(ctx context.Context, texts []string, embedders ...Embedder) ([][]float32, error) {
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
			return vecs, nil
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
		return true
	default:
		return false
	}
}

func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
