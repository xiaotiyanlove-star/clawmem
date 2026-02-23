package core

import (
	"context"
	"encoding/json"
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
1. Extract factual information (server IP, personal details, system states) and user preferences.
2. If two memories contradict each other, keep the NEWER one and note the change.
3. Merge related fragments into single coherent summaries.
4. If there is nothing worth remembering, output exactly: NOTHING_TO_CONSOLIDATE
5. MUST output in the following JSON format ONLY, without any markdown code block wrap:
{
  "consolidated": ["Summary of the event 1", "Summary of the event 2"],
  "preferences": [
    {"text": "User loves Go programming", "type": "explicit"}
  ],
  "facts": [
    {"text": "Server IP is 5.6.7.8", "supersedes": "1.2.3.4"}
  ]
}`

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

	// 0. 执行生命周期清理与存储预算控制 (即便是 DreamDisabled 也可能会被别处触发, 当前挂载在 Dream 周期内执行)
	if cfg.MemoryMaxCount > 0 {
		deleted, err := s.sqlStore.EnforceMemoryBudget(cfg.MemoryMaxCount)
		if err == nil && deleted > 0 {
			log.Printf("[AUDIT] Deleted %d low-value memories due to MAX_MEMORY_COUNT budget (%d)", deleted, cfg.MemoryMaxCount)
		} else if err != nil {
			log.Printf("[WARN] EnforceMemoryBudget failed: %v", err)
		}
	}

	// 清理 30 天未命中的废弃原始对话 (根据衰减规则: 30天未被命中 + access_count < 3 -> 自动软删除)
	expired, err := s.sqlStore.CleanExpiredConversations(30, 3)
	if err == nil && expired > 0 {
		log.Printf("[AUDIT] Cleaned %d expired conversation memories (inactive for >30 days, access <3)", expired)
	} else if err != nil {
		log.Printf("[WARN] CleanExpiredConversations failed: %v", err)
	}

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

	// 为了保证多租户与多会话读写隔离，Dream 必须按 user_id 分组隔离处理
	userG := make(map[string][]*model.Memory)
	for _, m := range memories {
		userG[m.UserID] = append(userG[m.UserID], m)
	}

	systemPrompt := cfg.DreamPrompt
	if systemPrompt == "" {
		systemPrompt = defaultDreamPrompt
	}

	type DreamOutput struct {
		Consolidated []string `json:"consolidated"`
		Preferences  []struct {
			Text string `json:"text"`
		} `json:"preferences"`
		Facts []struct {
			Text       string `json:"text"`
			Supersedes string `json:"supersedes"`
		} `json:"facts"`
	}

	totalOutput := 0
	hasError := false

	for userID, uMems := range userG {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("The following are %d raw memory fragments collected from user '%s'. Please consolidate them:\n\n", len(uMems), userID))
		for i, m := range uMems {
			sb.WriteString(fmt.Sprintf("[%d] (%s) %s\n", i+1, m.CreatedAt.Format("2006-01-02 15:04"), m.Content))
		}

		result, err := s.llmClient.Chat(ctx, systemPrompt, sb.String(), cfg.DreamLLMBase, cfg.DreamLLMKey, cfg.DreamLLMModel)
		if err != nil {
			log.Printf("[DREAM] LLM call failed for user %s: %v", userID, err)
			hasError = true
			continue
		}

		if strings.TrimSpace(result) == "NOTHING_TO_CONSOLIDATE" {
			log.Printf("[DREAM] User %s: LLM determined nothing worth consolidating.", userID)
			continue
		}

		// 移除可能的 Markdown 包装
		result = strings.TrimPrefix(strings.TrimSpace(result), "```json")
		result = strings.TrimSuffix(result, "```")
		result = strings.TrimSpace(result)

		var output DreamOutput
		err = json.Unmarshal([]byte(result), &output)

		// Resilience: JSON 解析降级
		if err != nil {
			log.Printf("[WARN] LLM JSON parsing failed for user %s, degraded to text extraction. parsing error: %v", userID, err)

			// 降级使用旧版文本按行拆分逻辑，存储为 kind=summary 退化兜底
			lines := strings.Split(result, "\n")
			for _, line := range lines {
				line = strings.TrimPrefix(strings.TrimSpace(line), "- ")
				if line != "" && !strings.HasPrefix(line, "{") && !strings.HasPrefix(line, "[") {
					output.Consolidated = append(output.Consolidated, line)
				}
			}
		}

		now := time.Now().UTC()
		dateTag := now.Format("2006-01-02")

		// 统一处理生成的记忆结果，为了简化向量并发，串行处理
		saveGenMem := func(content string, kind string, tags []string, supersedes string) {
			if content == "" {
				return
			}
			tags = append(tags, "dream", "consolidated", dateTag)

			// 如果有 supersedes，执行 set_memory 逻辑覆盖旧记录
			if supersedes != "" {
				tags = append(tags, "conflict_resolved")
				_, _ = s.SetMemory(ctx, &model.SetMemoryRequest{
					UserID:         userID,
					Content:        content,
					MatchQuery:     supersedes,
					MatchThreshold: 0.75,
					Kind:           kind,
					Source:         "dream",
					Tags:           tags,
				})
				totalOutput++
				return
			}

			// 否则直接写入
			vec, provider, vErr := s.embedManager.GetEmbedding(ctx, content)
			if vErr != nil {
				return
			}
			newID := uuid.New().String()
			mem := &model.Memory{
				ID:            newID,
				UserID:        userID,
				Content:       content,
				Source:        "dream",
				Kind:          kind,
				Tags:          tags,
				Status:        model.StatusDream,
				EmbedProvider: provider,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			_ = s.sqlStore.Insert(mem)
			_ = s.vectorStore.Add(ctx, mem.ID, content, map[string]string{
				"user_id": userID,
				"source":  "dream",
				"status":  model.StatusDream,
				"kind":    kind,
			}, vec)
			totalOutput++
		}

		for _, summ := range output.Consolidated {
			saveGenMem(summ, model.KindSummary, nil, "")
		}
		for _, pref := range output.Preferences {
			saveGenMem(pref.Text, model.KindPreference, nil, "")
		}
		for _, fact := range output.Facts {
			saveGenMem(fact.Text, model.KindFact, nil, fact.Supersedes)
		}

		// 标记原始记忆为已整合
		var idsToMark []string
		for _, m := range uMems {
			idsToMark = append(idsToMark, m.ID)
		}
		_ = s.sqlStore.MarkConsolidated(idsToMark)
	}

	statusMsg := ""
	if hasError {
		statusMsg = "completed with some user errors"
	}
	s.sqlStore.LogDreamFinish(logID, totalOutput, statusMsg)
	log.Printf("[DREAM] Dream cycle complete. Input: %d -> Output: %d memories.", len(memories), totalOutput)
	return nil
}
