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

	// 初始化向量存储
	vectorStore, err := storage.NewVectorStore(cfg, llmClient)
	if err != nil {
		log.Fatalf("初始化向量存储失败: %v", err)
	}

	// 初始化核心服务
	service := core.NewMemoryService(sqlStore, vectorStore, llmClient)

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
		mcp.WithDescription("Add a new memory or knowledge to the database. Use this when the user asks you to remember something."),
		mcp.WithString("content", mcp.Required(), mcp.Description("The content of the memory or knowledge")),
		mcp.WithString("source", mcp.Description("The source of the data (default: mcp)")),
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

		req := &model.AddMemoryRequest{
			UserID:  "default",
			Content: content,
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

		req := &model.SearchMemoryRequest{
			UserID: "default",
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

	// 启动 MCP Server (Stdio)
	log.SetOutput(os.Stderr)
	if err := server.ServeStdio(s); err != nil {
		log.Printf("Server error: %v", err)
	}
}
