package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/xiaotiyanlove-star/clawmem/config"
	"github.com/xiaotiyanlove-star/clawmem/internal/api"
	"github.com/xiaotiyanlove-star/clawmem/internal/core"
	"github.com/xiaotiyanlove-star/clawmem/internal/embedding"
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

	// 初始化 Embedding Manager
	embedManager := embedding.NewManager(cfg, sqlStore)

	// 初始化向量存储
	vectorStore, err := storage.NewVectorStore(cfg, embedManager)
	if err != nil {
		log.Fatalf("初始化向量存储失败: %v", err)
	}

	// 初始化核心服务
	service := core.NewMemoryService(cfg, sqlStore, vectorStore, llmClient, embedManager)

	// 启动 Dream 调度器（如果已启用）
	var dreamScheduler *core.DreamScheduler
	if cfg.DreamEnabled {
		dreamScheduler = core.NewDreamScheduler(service)
		dreamScheduler.Start()
	}

	// 启动自愈 (Healer) 调度器
	healerScheduler := core.NewHealerScheduler(service)
	healerScheduler.Start()

	// 初始化 HTTP 路由
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// 1. Payload 尺寸防御中间件 (DDoS 防御，限定 2MB)
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		c.Next()
	})

	// 2. 轻量鉴权中间件 (AuthZ)
	if cfg.AuthToken != "" {
		r.Use(func(c *gin.Context) {
			// 如果是健康检查或大屏看板，允许白名单匿名访问
			if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/dashboard" {
				c.Next()
				return
			}
			// 优先读取 Authorization 头，其次读取 X-API-KEY
			token := c.GetHeader("Authorization")
			if len(token) > 7 && token[:7] == "Bearer " {
				token = token[7:]
			}
			if token == "" {
				token = c.GetHeader("X-API-KEY")
			}
			if token != cfg.AuthToken {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid or missing API token"})
				c.Abort()
				return
			}
			c.Next()
		})
	}

	handler := api.NewHandler(service)
	handler.RegisterRoutes(r)

	// 优雅退出
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("收到退出信号，正在关闭服务...")
		if dreamScheduler != nil {
			dreamScheduler.Stop()
		}
		if healerScheduler != nil {
			healerScheduler.Stop()
		}
		os.Exit(0)
	}()

	log.Printf("ClawMem 服务启动，监听端口 :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
