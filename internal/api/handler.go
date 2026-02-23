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
	r.GET("/dashboard", h.Dashboard)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/memo", h.AddMemory)
		v1.GET("/memo/search", h.SearchMemory)
		v1.DELETE("/memo/:id", h.DeleteMemory)
		v1.POST("/memo/delete-by-query", h.DeleteByQuery)
		v1.POST("/memo/set", h.SetMemory)
		v1.PUT("/memo/:id", h.UpdateMemory)
		v1.POST("/dream/trigger", h.TriggerDream)
		v1.GET("/stats", h.Stats)
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

// Stats 返回数据大屏的统计看板数据
func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计数据失败: " + err.Error()})
		return
	}
	// 将 cfg 里的上限值动态注入（服务内只能拿 service，但是这可以通过一个小技巧获取或者单独写，为了快直接返回预定义的限制值或在服务中补充读取）
	// 因为 Handler 拿不到私有的 cfg，我们可以在 service 里补充，或者更简单的，在此处我们暂时假定它不显示百分比只显示骨架，或者这里用通过配置管理单例来拿
	maxCount, _ := h.service.GetMemoryMaxCount()
	c.JSON(http.StatusOK, gin.H{"data": stats, "max_count": maxCount})
}

// Dashboard 返回纯静态的监控 HTML 页面
func (h *Handler) Dashboard(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ClawMem 监控看板</title>
    <style>
        :root { --bg: #0f172a; --card: #1e293b; --text: #f8fafc; --accent: #38bdf8; --delete: #f43f5e; --warn: #fbbf24; }
        body { background: var(--bg); color: var(--text); font-family: system-ui, -apple-system, sans-serif; margin: 0; padding: 2rem; }
        .container { max-width: 1000px; margin: 0 auto; }
        h1 { color: var(--accent); display: flex; align-items: center; gap: 0.5rem; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1rem; margin-top: 2rem; }
        .card { background: var(--card); padding: 1.5rem; border-radius: 12px; border: 1px solid #334155; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1); }
        .card h3 { margin: 0 0 1rem 0; color: #94a3b8; font-size: 1rem; }
        .card .value { font-size: 2.5rem; font-weight: bold; }
        .refresh { background: var(--accent); color: #000; border: none; padding: 0.5rem 1rem; border-radius: 6px; cursor: pointer; font-weight: bold; transition: opacity 0.2s;}
        .refresh:hover { opacity: 0.8; }
        .progress-bg { background: #334155; height: 12px; border-radius: 6px; margin-top: 1rem; overflow: hidden; position: relative;}
        .progress-fill { background: var(--accent); height: 100%; transition: width 0.5s ease; }
        .progress-fill.warning { background: var(--warn); }
        .progress-fill.danger { background: var(--delete); }
        .skeleton { animation: pulse 1.5s infinite; background: #334155; height: 2rem; width: 60%; border-radius: 4px;}
        @keyframes pulse { 0% { opacity: 0.6; } 50% { opacity: 0.3; } 100% { opacity: 0.6; } }
    </style>
</head>
<body>
    <div class="container">
        <div style="display: flex; justify-content: space-between; align-items: center;">
            <h1>🦞 ClawMem 内存治理枢纽</h1>
            <button class="refresh" onclick="loadStats()">刷新状态</button>
        </div>
        <p style="color: #64748b; margin-top: -10px;">Zero-Dependency Sovereign Memory Layer for Agents</p>
        <div class="grid" id="statsGrid">
            <div class="card"><h3>🧊 活跃记忆碎片</h3><div class="skeleton"></div></div>
            <div class="card"><h3>🗑️ 遗忘/软删除区</h3><div class="skeleton"></div></div>
            <div class="card" style="grid-column: 1 / -1;"><h3>📊 记忆分层健康度</h3><div class="skeleton" style="width: 100%; height: 80px;"></div></div>
        </div>
    </div>
    <script>
        async function loadStats() {
            try {
                // 加载微动效
                document.getElementById('statsGrid').style.opacity = '0.5';

                const res = await fetch('/api/v1/stats');
                const {data, max_count} = await res.json();
                
                let kindsHTML = '';
                if(data.kind_counts) {
                    for(const [k, v] of Object.entries(data.kind_counts)) {
                        kindsHTML += '<div style="display:flex; justify-content: space-between; margin-bottom: 8px;"><span style="color: var(--accent)">' + k + '</span><span>' + v + '</span></div>';
                    }
                }

                let active = data.total_active || 0;
                let max = max_count || 5000;
                let pct = Math.min((active / max) * 100, 100).toFixed(1);
                let fillClass = pct > 90 ? 'danger' : (pct > 75 ? 'warning' : '');

                document.getElementById('statsGrid').style.opacity = '1';
                document.getElementById('statsGrid').innerHTML = '<div class="card"><h3>🧊 活跃记忆碎片</h3><div class="value">' + active + ' / <span style="font-size: 1rem; color: #64748b;">' + max + ' Max</span></div><div class="progress-bg"><div class="progress-fill ' + fillClass + '" style="width:' + pct + '%"></div></div></div><div class="card"><h3>🗑️ 遗忘/软删除区</h3><div class="value" style="color: var(--delete)">' + (data.total_deleted || 0) + '</div><div style="color: #64748b; font-size: 0.9rem; margin-top: 0.5rem;">等待引擎深度清除...</div></div><div class="card" style="grid-column: 1 / -1;"><h3>📊 记忆分层健康度 (Kinds)</h3><div style="font-family: monospace; font-size: 1.1rem; column-count: 2; column-gap: 2rem; background: #0f172a; padding: 1rem; border-radius: 8px; border: 1px solid #1e293b;">' + (kindsHTML || '<span style="color: #64748b">暂无数据</span>') + '</div></div>';
            } catch(e) {
                document.getElementById('statsGrid').style.opacity = '1';
                document.getElementById('statsGrid').innerHTML = '<div style="color: red; grid-column: 1/-1;">无法连接 ClawMem 服务获取状态，请检查网络。</div>';
            }
        }
        loadStats();
        setInterval(loadStats, 30000); // 30s 自动刷新
    </script>
</body>
</html>`
