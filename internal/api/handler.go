package api

import (
	"context"
	"log"
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
		v1.DELETE("/memo/:id", h.DeleteMemory)
		v1.POST("/memo/delete-by-query", h.DeleteByQuery)
		v1.POST("/memo/set", h.SetMemory)
		v1.PUT("/memo/:id", h.UpdateMemory)
		v1.POST("/dream/trigger", h.TriggerDream)
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
		log.Printf("[API ERROR] AddMemory payload validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 额外校验：防止前后端传入纯空格的内容骗过 required 标签
	if len(req.Content) == 0 {
		log.Printf("[API ERROR] AddMemory content is empty after binding")
		c.JSON(http.StatusBadRequest, gin.H{"error": "记忆内容不能为空"})
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
		log.Printf("[API ERROR] SearchMemory query parameter validation failed: %v, raw query: %s", err, c.Request.URL.RawQuery)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 如果 TopK 小于等于 0，给定一个合理的默认值
	if req.TopK <= 0 {
		req.TopK = 5
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

// DeleteMemory 根据 ID 删单条记忆
func (h *Handler) DeleteMemory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供 id"})
		return
	}
	if err := h.service.DeleteMemoryByID(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "指定记忆已删除"})
}

// DeleteByQuery 根据查询内容批量软删记忆
func (h *Handler) DeleteByQuery(c *gin.Context) {
	var req model.DeleteByQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	count, ids, err := h.service.DeleteMemoriesByQuery(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"deleted_count": count,
		"deleted_ids":   ids,
		"message":       "执行批量语义删除完成",
	})
}

// SetMemory 智能新增或覆盖修改
func (h *Handler) SetMemory(c *gin.Context) {
	var req model.SetMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if len(req.Content) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记忆内容不能为空"})
		return
	}

	mem, err := h.service.SetMemory(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "智能覆盖存储失败: " + err.Error()})
		return
	}

	opType := "inserted"
	if req.ID != "" && mem.ID == req.ID && mem.CreatedAt.Before(mem.UpdatedAt) {
		opType = "updated" // 这是个简略估计用于反馈，实际业务里也可以从 service 层传回标记
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "记忆处理完成",
		"operation": opType,
		"data":      mem,
	})
}

// UpdateMemory 直接基于 ID 简单更新记忆
func (h *Handler) UpdateMemory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供 id"})
		return
	}

	var req model.UpdateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if len(req.Content) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记忆内容不能为空"})
		return
	}

	mem, err := h.service.UpdateMemory(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "指定记忆已更新",
		"data":    mem,
	})
}

// TriggerDream 手动触发一次记忆整合
func (h *Handler) TriggerDream(c *gin.Context) {
	go func() {
		if err := h.service.RunDream(context.Background()); err != nil {
			log.Printf("[DREAM API] Error: %v", err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Dream cycle triggered. Check logs for progress.",
	})
}
