# ClawMem 使用指南 🦞

**ClawMem** 是一个专为 OpenClaw Agent 设计的轻量级、分级存储记忆服务，特别适合在资源受限环境（如低成本 VPS）上运行。

## 🌟 核心特性

*   **分级 Embedding 策略**:
    *   **Tier 1 (云端)**: 优先使用 **Cloudflare Workers AI** (免费额度) 或 OpenAI，获取高质量向量。
    *   **Tier 0 (本地兜底)**: 当云端 API 不可用时，自动降级到本地 Mock 模式（伪向量），确保服务永不崩溃。
*   **延迟加载 (Lazy Loading)**: 本地模型仅在必要时加载，正常运行时节省 ~200MB 内存。
*   **零 CGO**: 纯 Go 实现（含 SQLite），部署只需复制一个二进制文件。
*   **差量批处理**: 智能缓存未命中的文本，大幅减少 API 开销。

## 🚀 部署指南

### 1. 安装

**源码编译 (推荐)**

需要 Go 1.23+:

```bash
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
go build -o clawmem ./cmd/server
sudo mv clawmem /usr/local/bin/
```

### 2. 配置 (`/etc/clawmem/config.env`)

创建配置文件，建议优先使用 Cloudflare 免费层。

**如何获取 Cloudflare 配置：**
1.  登录 Cloudflare Dashboard -> User Profile -> API Tokens。
2.  创建 Token -> 选择模板 **"Workers AI"** (Read/Write)。
3.  复制 Token 到 `CF_API_TOKEN`。
4.  在 Workers 页面复制 Account ID。

```bash
# 端口
PORT=8090
# 数据存储路径
DB_PATH=/var/lib/clawmem/clawmem.db
VECTOR_DB_PATH=/var/lib/clawmem/vectors

# 策略: cloud_first (推荐), accuracy_first, 或 local_only
EMBEDDING_STRATEGY=cloud_first

# Cloudflare 配置
CF_ACCOUNT_ID=你的AccountID
CF_API_TOKEN=你的APIToken

# 可选: LLM 摘要配置
DISABLE_LLM_SUMMARY=true
```

### 3. 设置 Systemd 服务

创建文件 `/etc/systemd/system/clawmem.service`:

```ini
[Unit]
Description=ClawMem Memory Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/clawmem
ExecStart=/usr/local/bin/clawmem
Restart=always
RestartSec=5
EnvironmentFile=/etc/clawmem/config.env

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo mkdir -p /var/lib/clawmem
sudo systemctl enable --now clawmem
```

## 🔌 OpenClaw 接入 (技能模式)

这是最推荐的接入方式，不需要修改 OpenClaw 核心配置。

### 安装技能

将本项目中的 `skills/clawmem` 文件夹复制到你的 OpenClaw 技能目录（例如 `/root/.openclaw/workspace/skills/`）。

目录结构应如下：
```text
skills/
  └── clawmem/
      ├── SKILL.md
      └── client.py
```

### 依赖安装

```bash
pip install requests
```

### 如何使用

Agent 现在可以通过自然语言调用记忆功能：

*   **存储**: “帮我记住：Racknerd 的 SSH 端口是 11022”
    *   自动调用 `python client.py add ...`
*   **回忆**: “我之前存的 VPS 端口是多少？”
    *   自动调用 `python client.py search ...`

## 🛠️ 常见问题 (FAQ)

**Q: 需要部署 Cloudflare Worker 脚本吗？**
A: **不需要。** ClawMem 直接调用 Cloudflare Workers AI 的 REST API。你只需要申请一个 Token。

**Q: 为什么本地兜底是 Mock 模式？**
A: 在 2GB 内存的 VPS 上跑完整的 BERT 模型容易导致 OOM（内存溢出）。为了保证 OpenClaw 主进程的安全，我们默认在 API 挂掉时使用确定性哈希生成伪向量。这保证了服务活着，虽然此时搜索精度会下降。

**Q: 数据库怎么备份？**
A: 整个数据库就是一个文件 `/var/lib/clawmem/clawmem.db`。你可以用 cron 任务定期把它复制到你的 OneDrive 挂载目录。
