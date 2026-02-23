package core

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/xiaotiyanlove-star/clawmem/config"
	"github.com/xiaotiyanlove-star/clawmem/internal/embedding"
	"github.com/xiaotiyanlove-star/clawmem/internal/llm"
	"github.com/xiaotiyanlove-star/clawmem/internal/model"
	"github.com/xiaotiyanlove-star/clawmem/internal/storage"
)

func setupTestEnvironment(t *testing.T) (*MemoryService, func()) {
	t.Helper()

	dbPath := fmt.Sprintf("/tmp/clawmem_test_%d.db", time.Now().UnixNano())
	vecPath := fmt.Sprintf("/tmp/clawmem_vec_%d", time.Now().UnixNano())

	cfg := &config.Config{
		DBPath:            dbPath,
		VectorDBPath:      vecPath,
		EmbedDimension:    1024,
		EmbeddingStrategy: "local_only", // Use local only to avoid API calls in tests
		DisableLLMSummary: true,         // Avoid open AI summary calls
	}

	sqlStore, err := storage.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		t.Fatalf("Init SQLite Error: %v", err)
	}

	embedManager := embedding.NewManager(cfg, sqlStore)

	vectorStore, err := storage.NewVectorStore(cfg, embedManager)
	if err != nil {
		t.Fatalf("Init Vector Store Error: %v", err)
	}

	llmClient := llm.NewClient(cfg)
	service := NewMemoryService(cfg, sqlStore, vectorStore, llmClient, embedManager)

	cleanup := func() {
		sqlStore.Close()
		os.Remove(dbPath)
		os.RemoveAll(vecPath)
	}

	return service, cleanup
}

func TestSetMemory_ConcurrentRaceCondition(t *testing.T) {
	service, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user_race_1"
	content := "This is a race condition test"

	req := &model.SetMemoryRequest{
		UserID:         userID,
		Content:        content,
		MatchQuery:     content,
		MatchThreshold: 0.8,
	}

	// 模拟并发调用 SetMemory 覆盖同一个语义内容
	var wg sync.WaitGroup
	routines := 10
	results := make([]*model.Memory, routines)
	errors := make([]error, routines)

	for i := 0; i < routines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// 并发插入同样的 content，触发软删旧记录和插入新记录竞争
			mem, err := service.SetMemory(ctx, req)
			results[idx] = mem
			errors[idx] = err
		}(i)
	}
	wg.Wait()

	// 验证：最后系统中活跃的、具有该语义的记忆，由于并发机制，最起码应没有死锁崩溃，并在多次替换后依然能提供有效记录。
	// (在乐观 Delete+Add 的实现下，并发会有一定的乱序和互相替换，不追求绝对唯一但应全部成功且查询有结果)
	var successCount int
	for _, e := range errors {
		if e == nil {
			successCount++
		}
	}
	if successCount < routines {
		t.Errorf("Expected all %d routines to succeed, got %d. Check error: %v", routines, successCount, errors)
	}

	// 检索应返回最新有效数据
	searchReq := &model.SearchMemoryRequest{UserID: userID, Query: content}
	sRes, err := service.SearchMemory(ctx, searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(sRes) == 0 {
		t.Error("No active memories found after concurrent SetMemory")
	}
}

func TestTieredSearch_Isolation_Fallback(t *testing.T) {
	service, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// 插入不同维度的记忆
	// User1 Pref
	_, _ = service.AddMemory(ctx, &model.AddMemoryRequest{
		UserID:  "user1",
		Kind:    model.KindPreference,
		Content: "I like Go programming",
	})
	// User1 Summary
	_, _ = service.AddMemory(ctx, &model.AddMemoryRequest{
		UserID:  "user1",
		Kind:    model.KindSummary,
		Content: "User wants to build a CLI tool",
	})
	// User1 Conversation
	_, _ = service.AddMemory(ctx, &model.AddMemoryRequest{
		UserID:  "user1",
		Kind:    model.KindConversation,
		Content: "Hello, what time is it?",
	})

	// User2 Pref (用于测试隔离)
	_, _ = service.AddMemory(ctx, &model.AddMemoryRequest{
		UserID:  "user2",
		Kind:    model.KindPreference,
		Content: "I like Python",
	})

	// 1. 测试 User1 的混合召回
	// 预期: 必须召回 preference，同时召回 summary(预过滤)，以及可能召回 fallback 的 conversation
	res1, err := service.SearchMemory(ctx, &model.SearchMemoryRequest{
		UserID: "user1",
		Query:  "Go CLI",
	})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	foundPref := false
	foundSumm := false
	foundUser2Data := false

	for _, r := range res1 {
		if r.Memory.Kind == model.KindPreference && r.Memory.UserID == "user1" {
			foundPref = true
			if r.Score != 1.0 {
				t.Errorf("Expected score 1.0 for preference, got %f", r.Score)
			}
		}
		if r.Memory.Kind == model.KindSummary && r.Memory.UserID == "user1" {
			foundSumm = true
		}
		if r.Memory.UserID == "user2" {
			foundUser2Data = true
		}
	}

	if !foundPref {
		t.Error("Failed to recall preference memory for user1")
	}
	if !foundSumm {
		t.Error("Failed to recall summary memory for user1 via keyword filtering")
	}
	if foundUser2Data {
		t.Error("Isolation leak: recalled user2 data for user1 search")
	}

	t.Logf("Search results test passed: %d items retrieved in correct tiers.", len(res1))
}
