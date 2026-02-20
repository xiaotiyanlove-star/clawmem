package core

import (
	"context"
	"log"
	"time"
)

// HealerScheduler 负责后台自动修复使用了 Local 模型降级的记忆向量
type HealerScheduler struct {
	service *MemoryService
	ticker  *time.Ticker
	quit    chan struct{}
}

// NewHealerScheduler 创建后台修复调度器
func NewHealerScheduler(service *MemoryService) *HealerScheduler {
	return &HealerScheduler{
		service: service,
		quit:    make(chan struct{}),
	}
}

// Start 启动后台调度器
func (h *HealerScheduler) Start() {
	// 每 5 分钟巡检一次
	h.ticker = time.NewTicker(5 * time.Minute)
	go func() {
		log.Println("[INFO] Background Healer Scheduler started (interval: 5m)")
		for {
			select {
			case <-h.ticker.C:
				if err := h.service.RunHeal(context.Background()); err != nil {
					log.Printf("[WARN] Background Heal failed: %v", err)
				}
			case <-h.quit:
				h.ticker.Stop()
				log.Println("[INFO] Background Healer Scheduler stopped")
				return
			}
		}
	}()
}

// Stop 停止后台调度器
func (h *HealerScheduler) Stop() {
	close(h.quit)
}

// RunHeal 执行一次向量修复逻辑
func (s *MemoryService) RunHeal(ctx context.Context) error {
	// 每次最多处理 50 条 Local 记忆，避免触发 Cloudflare 或 OpenAI 速率限制
	batchSize := 50
	memories, err := s.sqlStore.GetLocalMemories(batchSize)
	if err != nil {
		return err
	}

	if len(memories) == 0 {
		return nil // 无需修复
	}

	log.Printf("[INFO] Healer found %d local memories. Attempting to heal with Cloud provider...", len(memories))

	// 提取原文本
	texts := make([]string, len(memories))
	for i, m := range memories {
		texts[i] = m.Content
		if m.Summary != "" {
			texts[i] = m.Summary // 如果有 summary，我们在存入 vector store 时使用的是 summary
		}
	}

	// 强制尝试云端进行 Batch Embedding 计算并覆写本地 Cache
	vecs, provider, err := s.embedManager.ForceCloudEmbeddingBatch(ctx, texts)
	if err != nil {
		log.Printf("[WARN] Healer strategy cloud_first failed (network likely still offline): %v", err)
		return nil // 如果强制调用云端失败，说明还处于断网/降级状态，放弃本次 heal，等待下一次循环
	}

	// 成功获取了云端高质量向量，开始更新向量库和 SQLite 标签
	successCount := 0
	for i, m := range memories {
		metadata := map[string]string{
			"user_id":    m.UserID,
			"session_id": m.SessionID,
			"source":     m.Source,
		}

		// Chromem-go 的 AddDocument 基于 ID 是 Upsert 语义，所以会覆盖旧向量
		if err := s.vectorStore.Add(ctx, m.ID, texts[i], metadata, vecs[i]); err != nil {
			log.Printf("[WARN] Healer failed to update vector db for memory %s: %v", m.ID, err)
			continue
		}

		// 更新 SQLite 中的 embed_provider
		if err := s.sqlStore.UpdateMemoryProvider(m.ID, provider); err != nil {
			log.Printf("[WARN] Healer failed to update sqlite provider for memory %s: %v", m.ID, err)
			continue
		}

		successCount++
	}

	log.Printf("[INFO] Healer successfully healed %d/%d memories. New provider: %s", successCount, len(memories), provider)
	return nil
}
