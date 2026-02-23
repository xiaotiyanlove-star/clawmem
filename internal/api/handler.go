package api

import (
	"context"
	"fmt"
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
		v1.GET("/memos", h.ListMemories)
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
	maxCount, _ := h.service.GetMemoryMaxCount()
	c.JSON(http.StatusOK, gin.H{"data": stats, "max_count": maxCount})
}

// ListMemories 返回记忆的分页列表 (Dashboard 详情使用)
func (h *Handler) ListMemories(c *gin.Context) {
	kind := c.Query("kind")
	limit := 50
	offset := 0

	// 简单的分页参数解析
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := c.Query("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	memos, err := h.service.GetMemories(kind, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": memos})
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
        :root { --bg: #0f172a; --card: #1e293b; --text: #f8fafc; --accent: #38bdf8; --delete: #f43f5e; --warn: #fbbf24; --success: #22c55e; }
        body { background: var(--bg); color: var(--text); font-family: system-ui, -apple-system, sans-serif; margin: 0; padding: 2rem; }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { color: var(--accent); display: flex; align-items: center; gap: 0.5rem; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1rem; margin-top: 2rem; }
        .card { background: var(--card); padding: 1.5rem; border-radius: 12px; border: 1px solid #334155; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1); cursor: pointer; transition: transform 0.2s; }
        .card:hover { transform: translateY(-2px); border-color: var(--accent); }
        .card h3 { margin: 0 0 1rem 0; color: #94a3b8; font-size: 1rem; }
        .card .value { font-size: 2.5rem; font-weight: bold; }
        .refresh { background: var(--accent); color: #000; border: none; padding: 0.5rem 1rem; border-radius: 6px; cursor: pointer; font-weight: bold; transition: opacity 0.2s;}
        .refresh:hover { opacity: 0.8; }
        .progress-bg { background: #334155; height: 12px; border-radius: 6px; margin-top: 1rem; overflow: hidden; position: relative;}
        .progress-fill { background: var(--accent); height: 100%; transition: width 0.5s ease; }
        .progress-fill.warning { background: var(--warn); }
        .progress-fill.danger { background: var(--delete); }
        
        /* 表格样式 */
        .section-title { margin-top: 3rem; display: flex; justify-content: space-between; align-items: center; }
        .table-container { background: var(--card); border-radius: 12px; border: 1px solid #334155; margin-top: 1rem; overflow: hidden; }
        table { width: 100%; border-collapse: collapse; text-align: left; }
        th { background: #0f172a; color: #94a3b8; font-weight: 500; padding: 1rem; font-size: 0.9rem; border-bottom: 1px solid #334155; }
        td { padding: 1rem; border-bottom: 1px solid #1e293b; font-size: 0.95rem; }
        tr:last-child td { border-bottom: none; }
        tr:hover td { background: #2d3a4f; }
        .badge { padding: 2px 8px; border-radius: 4px; font-size: 0.8rem; font-weight: bold; text-transform: uppercase; }
        .badge-conversation { background: #334155; color: #cbd5e1; }
        .badge-fact { background: #1e3a8a; color: #93c5fd; }
        .badge-preference { background: #3f1e1e; color: #fecaca; }
        .badge-summary { background: #064e3b; color: #6ee7b7; }
        .btn-view { color: var(--accent); cursor: pointer; text-decoration: underline; font-size: 0.9rem; }

        /* 弹窗样式 */
        .modal { display: none; position: fixed; z-index: 100; left: 0; top: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.8); backdrop-filter: blur(4px); }
        .modal-content { background: var(--card); margin: 5% auto; padding: 2rem; border-radius: 16px; width: 70%; max-width: 800px; border: 1px solid #475569; position: relative; }
        .close { position: absolute; right: 1.5rem; top: 1rem; font-size: 2rem; cursor: pointer; color: #94a3b8; }
        .modal-body { margin-top: 1.5rem; line-height: 1.6; }
        pre { background: #0f172a; padding: 1.2rem; border-radius: 8px; overflow-x: auto; color: #e2e8f0; white-space: pre-wrap; word-break: break-all; border: 1px solid #1e293b; }

        .skeleton { animation: pulse 1.5s infinite; background: #334155; height: 1.5rem; border-radius: 4px; }
        @keyframes pulse { 0% { opacity: 0.6; } 50% { opacity: 0.3; } 100% { opacity: 0.6; } }
    </style>
</head>
<body>
    <div class="container">
        <div style="display: flex; justify-content: space-between; align-items: center;">
            <h1><span id="logo">🦞</span> ClawMem 内存治理枢纽</h1>
            <button class="refresh" onclick="refreshAll()">同步刷新</button>
        </div>
        <p style="color: #64748b; margin-top: -10px;">Zero-Dependency Sovereign Memory Layer for Agents</p>
        
        <div class="grid" id="statsGrid">
            <div class="card" onclick="filterByKind('')"><h3>🧊 活跃记忆总数</h3><div class="skeleton"></div></div>
            <div class="card"><h3>🗑️ 遗忘/软删除度</h3><div class="skeleton"></div></div>
            <div class="card" style="grid-column: 1 / -1;"><h3>📊 分层存储统计</h3><div class="skeleton" style="height: 60px;"></div></div>
        </div>

        <div class="section-title">
            <h2 id="listTitle">最近记忆详情</h2>
            <div id="filterStatus" style="color: var(--accent); font-weight: bold;"></div>
        </div>

        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th style="width: 15%">创建时间</th>
                        <th style="width: 10%">Kind</th>
                        <th style="width: 60%">记忆片段 (Preview)</th>
                        <th style="width: 15%">操作</th>
                    </tr>
                </thead>
                <tbody id="memoList">
                    <tr><td colspan="4" style="text-align:center; color:#64748b; padding: 3rem;">正在加载记忆序列...</td></tr>
                </tbody>
            </table>
        </div>
    </div>

    <!-- 弹窗 -->
    <div id="detailModal" class="modal">
        <div class="modal-content">
            <span class="close" onclick="closeModal()">&times;</span>
            <h2 id="modalKind" style="margin-top: 0;"></h2>
            <div class="modal-body">
                <p style="color: #94a3b8; font-size: 0.9rem;">ID: <span id="modalID"></span> | User: <span id="modalUser"></span></p>
                <h3>原始内容:</h3>
                <pre id="modalContent"></pre>
                <div id="modalMetaZone">
                    <h3>语义摘要:</h3>
                    <pre id="modalSummary"></pre>
                </div>
            </div>
        </div>
    </div>

    <script>
        let currentKind = '';
        let memoCache = {};

        async function loadStats() {
            try {
                const res = await fetch('/api/v1/stats');
                const {data, max_count} = await res.json();
                
                const grid = document.getElementById('statsGrid');
                grid.innerHTML = '';

                let active = data.total_active || 0;
                let max = max_count || 5000;
                let pct = Math.min((active / max) * 100, 100).toFixed(1);
                let fillClass = pct > 90 ? 'danger' : (pct > 75 ? 'warning' : '');

                // Card 1
                let c1 = document.createElement('div');
                c1.className = 'card';
                c1.onclick = function() { filterByKind(''); };
                c1.innerHTML = '<h3>🧊 活跃记忆总数</h3><div class="value">' + active + ' <span style="font-size: 1rem; color: #64748b;">/ ' + max + '</span></div><div class="progress-bg"><div class="progress-fill ' + fillClass + '" style="width:' + pct + '%"></div></div>';
                grid.appendChild(c1);

                // Card 2
                let c2 = document.createElement('div');
                c2.className = 'card';
                c2.innerHTML = '<h3>🗑️ 遗忘/软删除区</h3><div class="value" style="color: var(--delete)">' + (data.total_deleted || 0) + '</div><div style="color: #64748b; font-size: 0.9rem; margin-top: 0.5rem;">等待引擎后台深度物理抹除...</div>';
                grid.appendChild(c2);

                // Card 3
                let c3 = document.createElement('div');
                c3.className = 'card';
                c3.style.gridColumn = '1 / -1';
                c3.style.display = 'flex';
                c3.style.alignItems = 'center';
                c3.style.justifyContent = 'space-around';
                c3.style.padding = '1rem';
                
                if (data.kind_counts && Object.keys(data.kind_counts).length > 0) {
                    for(const [k, v] of Object.entries(data.kind_counts)) {
                        let kd = document.createElement('div');
                        kd.style.flex = '1';
                        kd.style.textAlign = 'center';
                        kd.style.borderRight = '1px solid #334155';
                        kd.style.padding = '0 10px';
                        kd.style.cursor = 'pointer';
                        kd.onclick = function(e) {
                            e.stopPropagation();
                            filterByKind(k);
                        };
                        
                        let label = document.createElement('div');
                        label.style.color = '#94a3b8';
                        label.style.fontSize = '0.8rem';
                        label.style.marginBottom = '5px';
                        label.textContent = k.toUpperCase();
                        kd.appendChild(label);
                        
                        let val = document.createElement('div');
                        val.style.fontWeight = 'bold';
                        val.style.color = 'var(--accent)';
                        val.textContent = v;
                        kd.appendChild(val);
                        
                        c3.appendChild(kd);
                    }
                } else {
                    let span = document.createElement('span');
                    span.style.color = '#64748b';
                    span.textContent = '暂无分层数据';
                    c3.appendChild(span);
                }
                grid.appendChild(c3);

                // Card 1
                let c1 = document.createElement('div');
                c1.className = 'card';
                c1.onclick = function() { filterByKind(''); };
                c1.innerHTML = '<h3>🧊 活跃记忆总数</h3><div class="value">' + active + ' <span style="font-size: 1rem; color: #64748b;">/ ' + max + '</span></div><div class="progress-bg"><div class="progress-fill ' + fillClass + '" style="width:' + pct + '%"></div></div>';
                grid.appendChild(c1);

                // Card 2
                let c2 = document.createElement('div');
                c2.className = 'card';
                c2.innerHTML = '<h3>🗑️ 遗忘/软删除区</h3><div class="value" style="color: var(--delete)">' + (data.total_deleted || 0) + '</div><div style="color: #64748b; font-size: 0.9rem; margin-top: 0.5rem;">等待引擎后台深度物理抹除...</div>';
                grid.appendChild(c2);

                // Card 3
                let c3 = document.createElement('div');
                c3.className = 'card';
                c3.style.gridColumn = '1 / -1';
                c3.style.display = 'flex';
                c3.style.alignItems = 'center';
                c3.style.justifyContent = 'space-around';
                c3.style.padding = '1rem';
                c3.innerHTML = kindsHTML || '<span style="color: #64748b">暂无分层数据</span>';
                grid.appendChild(c3);

            } catch(e) {
                console.error(e);
            }
        }

        async function loadMemos(kind = '') {
            currentKind = kind;
            document.getElementById('filterStatus').innerText = kind ? '已过滤: ' + kind : '';
            const tbody = document.getElementById('memoList');
            tbody.style.opacity = '0.5';

            try {
                const res = await fetch('/api/v1/memos?kind=' + kind + '&limit=20');
                const {data} = await res.json();
                
                tbody.style.opacity = '1';
                if(!data || data.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="4" style="text-align:center; color:#64748b; padding: 3rem;">暂无记忆数据</td></tr>';
                    return;
                }

                data.forEach(m => { memoCache[m.id] = m; });

                tbody.innerHTML = '';
                data.forEach(m => {
                    const date = new Date(m.created_at).toLocaleString();
                    const rawContent = String(m.content || '');
                    const previewText = rawContent.length > 80 ? rawContent.substring(0, 80) + '...' : rawContent;
                    const kindStr = m.kind || 'CONV';
                    const kindClass = 'badge-' + (m.kind || 'conversation');

                    let tr = document.createElement('tr');
                    
                    let td1 = document.createElement('td');
                    td1.style.color = '#94a3b8';
                    td1.textContent = date;
                    tr.appendChild(td1);

                    let td2 = document.createElement('td');
                    let spanK = document.createElement('span');
                    spanK.className = 'badge ' + kindClass;
                    spanK.textContent = kindStr;
                    td2.appendChild(spanK);
                    tr.appendChild(td2);

                    let td3 = document.createElement('td');
                    td3.textContent = previewText;
                    tr.appendChild(td3);

                    let td4 = document.createElement('td');
                    let spanV = document.createElement('span');
                    spanV.className = 'btn-view';
                    spanV.textContent = '查看详情';
                    spanV.onclick = function() { showDetailById(m.id); };
                    td4.appendChild(spanV);
                    tr.appendChild(td4);

                    tbody.appendChild(tr);
                });
            } catch(e) {
                tbody.style.opacity = '1';
                tbody.innerHTML = '<tr><td colspan="4" style="text-align:center; color:red;">加载列表异常</td></tr>';
            }
        }

        function showDetailById(id) {
            const m = memoCache[id];
            if(!m) return;
            document.getElementById('modalKind').innerText = '记忆详情 (' + (m.kind || 'CONV') + ')';
            document.getElementById('modalID').innerText = m.id;
            document.getElementById('modalUser').innerText = m.user_id + (m.session_id ? ' @ '+m.session_id : '');
            document.getElementById('modalContent').innerText = m.content;
            if(m.summary) {
                document.getElementById('modalMetaZone').style.display = 'block';
                document.getElementById('modalSummary').innerText = m.summary;
            } else {
                document.getElementById('modalMetaZone').style.display = 'none';
            }
            document.getElementById('detailModal').style.display = "block";
        }

        function closeModal() {
            document.getElementById('detailModal').style.display = "none";
        }

        function filterByKind(k) {
            loadMemos(k);
        }

        function refreshAll() {
            loadStats();
            loadMemos(currentKind);
        }

        window.onclick = function(event) {
            if (event.target == document.getElementById('detailModal')) {
                closeModal();
            }
        }

        loadStats();
        loadMemos();
    </script>
</body>
</html>`
