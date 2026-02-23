# 提取自设计文档的原型笔记

## File: clawmem-integration/CLawMem-快速参考.md

# ClawMem 快速参考

## 🔑 认证
- **Dashboard Basic Auth** (访问网页和只读 API)
  - 用户: `admin`
  - 密码: `ClawMem@2025`
- **API Bearer Token** (写操作)
  - Token: `umA4BMNKkZVevPhMO11Jd6M7/nyLvEMfr6Z7XmWr8NU=`

## 🌐 地址
- 本地: http://localhost:8090/dashboard (需 Basic Auth)
- 公网: https://clawmem.38680053.xyz/dashboard (需 Basic Auth)
- 健康检查: /health (公开)

## 🛠️ OpenClaw 环境变量
```bash
export CLAWMEM_AUTH_TOKEN="umA4BMNKkZVevPhMO11Jd6M7/nyLvEMfr6Z7XmWr8NU="
export CLAWMEM_URL="https://clawmem.38680053.xyz/api/v1"
```

## 📊 数据状态 (2026-02-23)
- 总记忆数: 59
- 活跃: 14
- 已删除: 45
- 分层: conversation:14

## ⚙️ 配置文件
`/etc/clawmem/config.env`
```ini
PORT=8090
AUTH_TOKEN=umA4BMNKkZVevPhMO11Jd6M7/nyLvEMfr6Z7XmWr8NU=
DASHBOARD_BASIC_AUTH=admin:ClawMem@2025
...
```

## 🧪 示例
```bash
# 搜索记忆 (带 token)
curl -H "Authorization: Bearer umA4BMNKkZVevPhMO11Jd6M7/nyLvEMfr6Z7XmWr8NU=" \
  "http://localhost:8090/api/v1/memo/search?user_id=default&query=VPS&top_k=3"

# 访问 Dashboard (带 Basic Auth)
curl -H "Authorization: Basic $(echo -n 'admin:ClawMem@2025' | base64)" \
  http://localhost:8090/dashboard
```

---
## File: clawmem-integration/CLawMem-部署记录.md

# ClawMem 部署记录

## 📦 服务信息
- **服务名**: ClawMem (内存治理服务)
- **主机**: Racknerd-2.5 (Debian 12)
- **监听端口**: 8090
- **进程管理**: systemd (`clawmem.service`)
- **二进制路径**: `/usr/local/bin/clawmem`
- **数据库**: SQLite `/var/lib/clawmem/clawmem.db`
- **向量库**: Chromem `/var/lib/clawmem/vectors`

## 🚀 代码版本
- **分支**: `feature/memory-soft-delete-set`
- **最新提交**: `ac90237` (2026-02-23 17:38)
- **编译时间**: 2026-02-23 18:03
- **版本号**: 0.1.0

## 🔐 认证配置 (当前生产)
### Dashboard Basic Auth
- **类型**: HTTP Basic Authentication
- **用户名**: `admin`
- **密码**: `ClawMem@2025`
- **配置位置**: `/etc/clawmem/config.env` 中 `DASHBOARD_BASIC_AUTH=admin:ClawMem@2025`
- **保护路径**:
  - `/dashboard`
  - `/api/v1/stats`
  - `/api/v1/memos`

### API Bearer Token (写操作)
- **Token**: `umA4BMNKkZVevPhMO11Jd6M7/nyLvEMfr6Z7XmWr8NU=`
- **Header**: `Authorization: Bearer <token>` 或 `X-API-KEY: <token>`
- **受影响接口**:
  - `POST /api/v1/memo`
  - `DELETE /api/v1/memo/:id`
  - `POST /api/v1/memo/delete-by-query`
  - `POST /api/v1/memo/set`
  - `PUT /api/v1/memo/:id`
  - `POST /api/v1/dream/trigger`

### 公开访问 (无鉴权)
- `GET /health`
- `GET /favicon.ico`

## 🌐 访问地址
### 本地
- Dashboard (需 Basic Auth): `http://localhost:8090/dashboard`
- API 健康检查: `http://localhost:8090/health`
- API 统计 (需 Basic Auth): `http://localhost:8090/api/v1/stats`
- API 列表 (需 Basic Auth): `http://localhost:8090/api/v1/memos?limit=20`

### 公网 (Cloudflare Tunnel)
- **Dashboard**: `https://clawmem.38680053.xyz/dashboard` (需 Basic Auth)
- Tunnel 配置独立，与 openclaw 的 tunnel 分离

## 🧪 快速测试
```bash
# Dashboard (需要 Basic Auth)
curl -H "Authorization: Basic $(echo -n 'admin:ClawMem@2025' | base64)" http://localhost:8090/dashboard | head -10

# Stats API (需要 Basic Auth)
curl -H "Authorization: Basic $(echo -n 'admin:ClawMem@2025' | base64)" http://localhost:8090/api/v1/stats | python3 -m json.tool

# 添加记忆 (需要 Bearer Token)
curl -H "Authorization: Bearer umA4BMNKkZVevPhMO11Jd6M7/nyLvEMfr6Z7XmWr8NU=" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"default","content":"测试记忆"}' \
  http://localhost:8090/api/v1/memo
```

## 🛠️ OpenClaw 集成
### 技能路径
`/root/.openclaw/workspace/skills/clawmem/client.py`

### 环境变量
```bash
# 用于 API 写操作鉴权
export CLAWMEM_AUTH_TOKEN="umA4BMNKkZVevPhMO11Jd6M7/nyLvEMfr6Z7XmWr8NU="

# 如果 OpenClaw 不在本机运行 clawmem，覆盖 URL
export CLAWMEM_URL="https://clawmem.38680053.xyz/api/v1"
```

### CLI 用法
```bash
python3 client.py search --user haibo --query "VPS" --auth-token umA4BMNK...
```
(如果不传 `--auth-token`，会读取 `CLAWMEM_AUTH_TOKEN`)

## 🔄 运维命令
```bash
# 查看状态
systemctl status clawmem

# 重启服务
systemctl restart clawmem

# 查看日志
journalctl -u clawmem -f

# 手动编译 (在主仓库)
cd /tmp/latest-clawmem
go build -o clawmem ./cmd/server/
cp clawmem /usr/local/bin/clawmem
systemctl restart clawmem
```

## 📊 数据库状态 (2026-02-23)
- **总记录数**: 59
- **活跃记忆**: 14
- **已软删除**: 45
- **分层分布**: conversation: 14

## ⚙️ 配置概览 (`/etc/clawmem/config.env`)
```ini
PORT=8090
DB_PATH=/var/lib/clawmem/clawmem.db
VECTOR_DB_PATH=/var/lib/clawmem/vectors
EMBEDDING_STRATEGY=cloud_first
AUTH_TOKEN=umA4BMNKkZVevPhMO11Jd6M7/nyLvEMfr6Z7XmWr8NU=
DASHBOARD_BASIC_AUTH=admin:ClawMem@2025

# Cloudflare Workers AI (Embedding)
CF_ACCOUNT_ID=7fdd96c2a530b3d10f0bfe923dbdf590
CF_API_TOKEN=Im2XciEjPc0UP7nkkRIgScCBLBHApx4oaziLT9EF

# LLM (OpenRouter)
LLM_API_BASE=https://openrouter.ai/api/v1
LLM_API_KEY=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
LLM_MODEL=stepfun/step-3.5-flash:free
DISABLE_LLM_SUMMARY=false

# Dream (自动整合)
DREAM_ENABLED=true
DREAM_INTERVAL=24h
DREAM_WINDOW=72h
DREAM_MIN_COUNT=5
DREAM_MAX_ITEMS=200
```

## 🔐 安全建议
- ✅ Dashboard 已加 Basic Auth
- ✅ API 写操作需 Bearer Token
- ✅ Cloudflare Tunnel 提供 HTTPS + 边缘加速
- 📌 可选：进一步启用 Cloudflare Access 做更强的身份验证
- 📌 建议定期备份 `/var/lib/clawmem/` 目录

## 🗓️ 最后更新
2026-02-23 18:03 (commit ac90237)

---
