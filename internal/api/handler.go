package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xiaotiyanlove-star/clawmem/internal/core"
	"github.com/xiaotiyanlove-star/clawmem/internal/model"
)

const version = "0.1.0"

// Handler HTTP 路由处理器
type Handler struct {
	service *core.MemoryService
}

// NewHandler 创建路由处理器
func NewHandler(service *core.MemoryService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册所有路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.Health)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/memo", h.AddMemory)
		v1.GET("/memo/search", h.SearchMemory)
	}
}

// Health 健康检查
func (h *Handler) Health(c *gin.Context) {
	count, _ := h.service.GetMemoryCount()
	c.JSON(http.StatusOK, model.HealthResponse{
		Status:      "ok",
		MemoryCount: count,
		Version:     version,
	})
}

// AddMemory 添加记忆
func (h *Handler) AddMemory(c *gin.Context) {
	var req model.AddMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	mem, err := h.service.AddMemory(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存储记忆失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "记忆已存储",
		"data":    mem,
	})
}

// SearchMemory 检索记忆
func (h *Handler) SearchMemory(c *gin.Context) {
	var req model.SearchMemoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	results, err := h.service.SearchMemory(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  results,
		"total": len(results),
	})
}
