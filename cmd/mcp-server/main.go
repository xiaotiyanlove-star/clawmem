package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/xiaotiyanlove-star/clawmem/config"
	"github.com/xiaotiyanlove-star/clawmem/internal/core"
	"github.com/xiaotiyanlove-star/clawmem/internal/embedding"
	"github.com/xiaotiyanlove-star/clawmem/internal/llm"
	"github.com/xiaotiyanlove-star/clawmem/internal/model"
	"github.com/xiaotiyanlove-star/clawmem/internal/storage"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化 LLM 客户端
	llmClient := llm.NewClient(cfg)

	// 初始化 SQLite 存储
	sqlStore, err := storage.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("初始化 SQLite 失败: %v", err)
	}
	defer sqlStore.Close()

	// 初始化 Embedding Manager
	embedManager := embedding.NewManager(cfg, sqlStore)

	// 初始化向量存储
	vectorStore, err := storage.NewVectorStore(cfg, embedManager)
	if err != nil {
		log.Fatalf("初始化向量存储失败: %v", err)
	}

	// 初始化核心服务
	service := core.NewMemoryService(cfg, sqlStore, vectorStore, llmClient, embedManager)

	// 创建 MCP Server
	s := server.NewMCPServer(
		"ClawMem",
		"0.1.0",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	// ---------------------------------------------------------
	// Tool: add_memory
	// ---------------------------------------------------------
	addMemoryTool := mcp.NewTool("add_memory",
		mcp.WithDescription("Add a new memory or knowledge to the database. Provide kind='preference' or 'fact' if appropriate."),
		mcp.WithString("content", mcp.Required(), mcp.Description("The content of the memory or knowledge")),
		mcp.WithString("kind", mcp.Description("Tier of memory ('preference', 'fact', 'conversation'). (default: conversation)")),
		mcp.WithString("source", mcp.Description("The source of the data (default: mcp)")),
		mcp.WithString("user_id", mcp.Description("The unique ID of the user or session (default: global_user)")),
	)

	s.AddTool(addMemoryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("arguments must be a map"), nil
		}

		content, ok := args["content"].(string)
		if !ok {
			return mcp.NewToolResultError("content must be a string"), nil
		}
		source, _ := args["source"].(string)
		if source == "" {
			source = "mcp"
		}
		userID, _ := args["user_id"].(string)
		if userID == "" {
			userID = "global_user"
		}
		kind, _ := args["kind"].(string)
		if kind == "" {
			kind = model.KindConversation
		}

		req := &model.AddMemoryRequest{
			UserID:  userID,
			Content: content,
			Kind:    kind,
			Source:  source,
		}

		mem, err := service.AddMemory(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to add memory: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Memory added with ID: %s", mem.ID)), nil
	})

	// ---------------------------------------------------------
	// Tool: search_memory
	// ---------------------------------------------------------
	searchMemoryTool := mcp.NewTool("search_memory",
		mcp.WithDescription("Search for relevant memories or knowledge. Use this before answering questions to retrieve context."),
		mcp.WithString("query", mcp.Required(), mcp.Description("The search query")),
		mcp.WithNumber("top_k", mcp.Description("Number of results to return (default: 5)")),
		mcp.WithString("user_id", mcp.Description("The unique ID of the user or session (default: global_user)")),
	)

	s.AddTool(searchMemoryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("arguments must be a map"), nil
		}

		query, ok := args["query"].(string)
		if !ok {
			return mcp.NewToolResultError("query must be a string"), nil
		}

		topK := 5.0
		if k, ok := args["top_k"].(float64); ok {
			topK = k
		}
		userID, _ := args["user_id"].(string)
		if userID == "" {
			userID = "global_user"
		}

		req := &model.SearchMemoryRequest{
			UserID: userID,
			Query:  query,
			TopK:   int(topK),
		}

		results, err := service.SearchMemory(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to search memory: %v", err)), nil
		}

		if len(results) == 0 {
			return mcp.NewToolResultText("No relevant memories found."), nil
		}

		var resp string
		for i, r := range results {
			resp += fmt.Sprintf("[%d] (Score: %.2f) %s\n", i+1, r.Score, r.Memory.Content)
			if r.Memory.Source != "" {
				resp += fmt.Sprintf("    Source: %s\n", r.Memory.Source)
			}
			resp += "\n"
		}

		return mcp.NewToolResultText(resp), nil
	})

	// ---------------------------------------------------------
	// Tool: set_memory
	// ---------------------------------------------------------
	setMemoryTool := mcp.NewTool("set_memory",
		mcp.WithDescription("Intelligently overwrite or store a new memory. Best for updating known facts or preferences without duplication."),
		mcp.WithString("content", mcp.Required(), mcp.Description("The new content to remember")),
		mcp.WithString("match_query", mcp.Description("Semantic query to find the old memory to overwrite (e.g. 'Old IP')")),
		mcp.WithString("kind", mcp.Description("Tier of memory ('preference', 'fact', 'conversation'). (default: fact)")),
		mcp.WithString("user_id", mcp.Description("The unique ID of the user or session (default: global_user)")),
	)

	s.AddTool(setMemoryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("arguments must be a map"), nil
		}
		content, ok := args["content"].(string)
		if !ok {
			return mcp.NewToolResultError("content must be a string"), nil
		}
		matchQuery, _ := args["match_query"].(string)
		userID, _ := args["user_id"].(string)
		if userID == "" {
			userID = "global_user"
		}
		kind, _ := args["kind"].(string)
		if kind == "" {
			kind = model.KindFact
		}

		req := &model.SetMemoryRequest{
			UserID:     userID,
			Content:    content,
			MatchQuery: matchQuery,
			Kind:       kind,
			Source:     "mcp",
		}
		mem, err := service.SetMemory(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to set memory: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Memory set successfully. ID: %s", mem.ID)), nil
	})

	// ---------------------------------------------------------
	// Tool: delete_memory
	// ---------------------------------------------------------
	deleteMemoryTool := mcp.NewTool("delete_memory",
		mcp.WithDescription("Delete a specific memory by its exact ID. Use search_memory first to find the ID if needed."),
		mcp.WithString("id", mcp.Required(), mcp.Description("The exact ID of the memory to delete")),
	)

	s.AddTool(deleteMemoryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("arguments must be a map"), nil
		}
		id, ok := args["id"].(string)
		if !ok {
			return mcp.NewToolResultError("id must be a string"), nil
		}

		err := service.DeleteMemoryByID(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete memory: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Successfully soft-deleted memory: %s", id)), nil
	})

	// ---------------------------------------------------------
	// Tool: get_preferences
	// ---------------------------------------------------------
	getPreferencesTool := mcp.NewTool("get_preferences",
		mcp.WithDescription("Get the explicit core preferences, traits and long-term rules for a user."),
		mcp.WithString("user_id", mcp.Description("The unique ID of the user or session (default: global_user)")),
		mcp.WithNumber("limit", mcp.Description("Number of preferences to return (default: 10)")),
	)

	s.AddTool(getPreferencesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("arguments must be a map"), nil
		}
		userID, _ := args["user_id"].(string)
		if userID == "" {
			userID = "global_user"
		}
		limit := 10
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}

		prefs, err := sqlStore.SearchPreferences(userID, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get preferences: %v", err)), nil
		}

		if len(prefs) == 0 {
			return mcp.NewToolResultText("No preferences found for user."), nil
		}

		var resp string
		for i, p := range prefs {
			resp += fmt.Sprintf("[%d] %s\n", i+1, p.Content)
		}
		return mcp.NewToolResultText(resp), nil
	})

	// 启动 MCP Server (Stdio)
	log.SetOutput(os.Stderr)
	if err := server.ServeStdio(s); err != nil {
		log.Printf("Server error: %v", err)
	}
}
