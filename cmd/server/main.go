package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/xiaotiyanlove-star/clawmem/config"
	"github.com/xiaotiyanlove-star/clawmem/internal/api"
	"github.com/xiaotiyanlove-star/clawmem/internal/core"
	"github.com/xiaotiyanlove-star/clawmem/internal/llm"
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

	// 初始化 HTTP 路由
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	handler := api.NewHandler(service)
	handler.RegisterRoutes(r)

	// 优雅退出
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("收到退出信号，正在关闭服务...")
		os.Exit(0)
	}()

	log.Printf("ClawMem 服务启动，监听端口 :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
