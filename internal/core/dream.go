package core

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xiaotiyanlove-star/clawmem/internal/model"
)

// 默认 Dream System Prompt
const defaultDreamPrompt = `You are a memory consolidation engine. Your job is to review a batch of raw memory fragments from an AI agent's daily interactions and produce a concise, high-quality set of consolidated memories.

Rules:
1. Extract only factual, actionable information (user preferences, decisions, technical facts, project status, important events).
2. Discard greetings, filler words, acknowledgments, and trivial chatter.
3. If two memories contradict each other, keep the NEWER one and note the change (e.g., "User preference updated: now prefers X over Y").
4. Merge related fragments into single coherent statements.
5. Each output line should be a standalone fact that makes sense without context.
6. Output one consolidated memory per line, prefixed with "- ".
7. If there is nothing worth remembering, output exactly: NOTHING_TO_CONSOLIDATE
8. Keep the original language of the memories (don't translate).
9. Maximum 20 consolidated memories per batch.`

// DreamScheduler 管理 Dream 定时任务的调度器
type DreamScheduler struct {
	service  *MemoryService
	interval time.Duration
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewDreamScheduler 创建 Dream 调度器
func NewDreamScheduler(service *MemoryService) *DreamScheduler {
	interval, err := time.ParseDuration(service.cfg.DreamInterval)
	if err != nil || interval <= 0 {
		interval = 24 * time.Hour
	}
	return &DreamScheduler{
		service:  service,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动定时调度（非阻塞）
func (ds *DreamScheduler) Start() {
	ds.mu.Lock()
	if ds.running {
		ds.mu.Unlock()
		return
	}
	ds.running = true
	ds.mu.Unlock()

	go ds.loop()
	log.Printf("[DREAM] Scheduler started. Interval: %s", ds.interval)
}

// Stop 停止调度器
func (ds *DreamScheduler) Stop() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if !ds.running {
		return
	}
	close(ds.stopCh)
	ds.running = false
	log.Println("[DREAM] Scheduler stopped.")
}

func (ds *DreamScheduler) loop() {
	ticker := time.NewTicker(ds.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("[DREAM] Scheduled dream cycle triggered.")
			if err := ds.service.RunDream(context.Background()); err != nil {
				log.Printf("[DREAM] Error: %v", err)
			}
		case <-ds.stopCh:
			return
		}
	}
}

// RunDream 执行一次记忆整合（可被定时器或 API 手动触发）
func (s *MemoryService) RunDream(ctx context.Context) error {
	cfg := s.cfg
	if !cfg.DreamEnabled {
		return fmt.Errorf("dream feature is disabled (set DREAM_ENABLED=true to enable)")
	}

	// 解析时间窗口
	window, err := time.ParseDuration(cfg.DreamWindow)
	if err != nil || window <= 0 {
		window = 24 * time.Hour
	}
	since := time.Now().UTC().Add(-window)

	maxItems := cfg.DreamMaxItems
	if maxItems <= 0 {
		maxItems = 200
	}

	// 1. 获取时间窗口内的活跃记忆
	memories, err := s.sqlStore.GetRecentActive(since, maxItems)
	if err != nil {
		return fmt.Errorf("failed to fetch recent memories: %w", err)
	}

	if len(memories) < cfg.DreamMinCount {
		log.Printf("[DREAM] Only %d memories found (min: %d). Skipping.", len(memories), cfg.DreamMinCount)
		return nil
	}

	log.Printf("[DREAM] Processing %d memories from the last %s...", len(memories), window)

	// 记录 Dream 日志
	logID, _ := s.sqlStore.LogDreamStart(time.Now().UTC(), len(memories))

	// 2. 组装 Prompt
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("The following are %d raw memory fragments collected in the last %s. Please consolidate them:\n\n", len(memories), window))
	for i, m := range memories {
		sb.WriteString(fmt.Sprintf("[%d] (%s) %s\n", i+1, m.CreatedAt.Format("2006-01-02 15:04"), m.Content))
	}

	systemPrompt := cfg.DreamPrompt
	if systemPrompt == "" {
		systemPrompt = defaultDreamPrompt
	}

	// 3. 调用 LLM
	result, err := s.llmClient.Chat(ctx, systemPrompt, sb.String(), cfg.DreamLLMBase, cfg.DreamLLMKey, cfg.DreamLLMModel)
	if err != nil {
		errMsg := fmt.Sprintf("LLM call failed: %v", err)
		s.sqlStore.LogDreamFinish(logID, 0, errMsg)
		return fmt.Errorf("dream LLM call failed: %w", err)
	}

	// 4. 解析结果
	if strings.TrimSpace(result) == "NOTHING_TO_CONSOLIDATE" {
		log.Println("[DREAM] LLM determined nothing worth consolidating.")
		s.sqlStore.LogDreamFinish(logID, 0, "")
		return nil
	}

	// 解析每一行 "- xxx" 为独立的精华记忆
	lines := strings.Split(result, "\n")
	var consolidated []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 去掉 "- " 前缀（如果有）
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		if line != "" {
			consolidated = append(consolidated, line)
		}
	}

	if len(consolidated) == 0 {
		log.Println("[DREAM] No consolidated memories extracted.")
		s.sqlStore.LogDreamFinish(logID, 0, "")
		return nil
	}

	log.Printf("[DREAM] Consolidated %d memories into %d entries.", len(memories), len(consolidated))

	// 5. 存入精华记忆
	now := time.Now().UTC()
	dateTag := now.Format("2006-01-02")
	for _, content := range consolidated {
		dreamMem := &model.Memory{
			ID:        uuid.New().String(),
			UserID:    "default",
			Content:   content,
			Source:    "dream",
			Tags:      []string{"dream", "consolidated", dateTag},
			Status:    model.StatusDream,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.sqlStore.Insert(dreamMem); err != nil {
			log.Printf("[DREAM] Failed to store consolidated memory: %v", err)
			continue
		}

		// 同时写入向量库，使精华记忆可被语义检索
		metadata := map[string]string{
			"user_id": dreamMem.UserID,
			"source":  "dream",
			"status":  model.StatusDream,
		}
		if err := s.vectorStore.Add(ctx, dreamMem.ID, content, metadata); err != nil {
			log.Printf("[DREAM] Failed to store vector for consolidated memory: %v", err)
		}
	}

	// 6. 标记原始记忆为已整合
	ids := make([]string, len(memories))
	for i, m := range memories {
		ids[i] = m.ID
	}
	if err := s.sqlStore.MarkConsolidated(ids); err != nil {
		log.Printf("[DREAM] Failed to mark memories as consolidated: %v", err)
	}

	s.sqlStore.LogDreamFinish(logID, len(consolidated), "")
	log.Printf("[DREAM] Dream cycle complete. %d -> %d memories.", len(memories), len(consolidated))
	return nil
}
