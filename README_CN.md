# ClawMem 🦞

**低成本 AI Agent 的「主权记忆层」。**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/xiaotiyanlove-star/clawmem)](https://goreportcard.com/report/github.com/xiaotiyanlove-star/clawmem)
[![Go Version](https://img.shields.io/github/go-mod/go-version/xiaotiyanlove-star/clawmem)](go.mod)

[🇬🇧 English Documentation](README.md)

---

## 💡 为什么需要 ClawMem？

通常，要让 AI Agent 拥有长期记忆，你需要部署一个 **向量数据库** 和一个 **Embedding 模型**。但对于运行在 **廉价 VPS ($5/月)** 上的个人 Agent 来说，这简直是灾难：

| 痛点 | 没有 ClawMem | 有了 ClawMem |
| :--- | :--- | :--- |
| **内存占用** | Docker + Python 向量库吃掉 500MB+ | 纯 Go 二进制，**<20MB** 内存 |
| **使用成本** | 每次请求都要付费调 OpenAI Embedding | **免费** Cloudflare Workers AI |
| **Token 消耗** | 每次对话都要把完整历史塞进上下文 | 只检索 **Top-K 条相关记忆** |
| **容错能力** | 单点故障，挂了就挂了 | 三层自动降级，永不崩溃 |
| **部署方式** | Docker Compose, Python, pip, venv... | **单文件**，零依赖 |

**ClawMem** 是专为低配、主权级 AI Agent 设计的**极轻量、高韧性记忆层**。

---

## ✨ 核心特性

- 🪶 **极致轻量** — 纯 Go 编写，静态编译。单文件约 **~15MB**，运行时内存 **<20MB**。最便宜的 VPS 也能跑。
- 💰 **零成本 Embedding** — 优先使用 Cloudflare Workers AI 免费层，提供高质量语义理解，无需花一分钱。
- 🛡️ **究极稳健** — 三层自动降级策略：Cloudflare → OpenAI 兼容 → 本地模型。断网不崩溃，限流不报错。
- ⚡ **智能缓存** — 内置 SQLite 语义缓存，支持部分缓存命中（差量计算）。重复文本 = 零 API 调用。
- 🔄 **批量处理** — 原生支持批量 Embedding，最大限度减少 HTTP 往返次数。
- 🔌 **MCP 协议** — 内置 MCP Server，可无缝接入 Claude Desktop、OpenClaw 等 MCP 客户端。
- 🧠 **延迟加载** — 本地模型按需加载，Cloud 模式下保持极低内存占用。
- 🏥 **启动自检** — 启动时自动检测 API 可用性，不可用的 Provider 立即标记为 DOWN，避免运行时超时。
- 💤 **梦境引擎 (Dream)** — 后台自动整合记忆。将碎片化的聊天记录压缩为高质量、无冲突的事实依据 (基于 LLM)。
- 🛠️ **自愈机制 (Healer)** — 自动将断网时生成的“本地方言”向量升级为云端高精度向量。彻底告别“幽灵数据”。

---

## 🏗️ 架构概览

```mermaid
graph TD
    User[OpenClaw / MCP 客户端] -->|存储 / 检索| API[HTTP API :8090]
    User -->|MCP 协议| MCP[MCP Server :stdio]
    API --> Service[核心服务]
    MCP --> Service
    Service -->|文本数据| SQLite[(SQLite DB<br/>原始文本 + 缓存)]
    Service -->|获取向量| Manager[Embedding 管理器]
    
    Dream[💤 梦境引擎<br/>后台整合任务] -.->|读取/压缩| SQLite
    Dream -.->|存储精华记忆| Service
    Dream -.->|调用提炼| LLM[🧠 LLM 服务商]

    Healer[🛠️ 自愈神医<br/>后台向量升级] -.->|升级本地向量| Manager
    Healer -.->|更新缓存| SQLite
    
    subgraph "多级 Embedding 策略"
        Manager -->|"Tier 1 · 主力"| CF[☁️ Cloudflare Workers AI<br/>免费 · 快速]
        Manager -->|"Tier 1 · 备选"| OA[🤖 OpenAI 兼容<br/>SiliconFlow 等]
        Manager -->|"Tier 0 · 兜底"| Local[💻 本地 BERT<br/>延迟加载 · 离线可用]
    end
    
    Manager -->|向量数据| VectorDB[(Chromem-go<br/>向量存储)]
    
    style CF fill:#f9f,stroke:#333
    style OA fill:#ffc,stroke:#333
    style Local fill:#cfc,stroke:#333
    style VectorDB fill:#bbf,stroke:#333
    style Dream fill:#fcf,stroke:#333,stroke-dasharray: 5 5
    style Healer fill:#cef,stroke:#333,stroke-dasharray: 5 5
    style LLM fill:#ff9,stroke:#333
```

---

## ⚡ 快速开始

### 方式一：下载预编译二进制

前往 [GitHub Releases](https://github.com/xiaotiyanlove-star/clawmem/releases) 下载最新的 Alpha 版本。

```bash
# Linux (amd64)
chmod +x clawmem-linux-amd64
./clawmem-linux-amd64

# macOS (Apple Silicon)
chmod +x clawmem-darwin-arm64
./clawmem-darwin-arm64
```

### 方式二：从源码编译

```bash
git clone https://github.com/xiaotiyanlove-star/clawmem.git
cd clawmem
CGO_ENABLED=0 go build -o clawmem ./cmd/server/
./clawmem
```

### 方式三：一键服务器部署

```bash
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
sudo ./scripts/install.sh
```

脚本会交互式询问服务端口、数据库路径和 Cloudflare 凭证，然后自动编译并注册 `systemd` 服务。

---

## 🔧 配置说明

通过环境变量或 `.env` 文件进行配置。完整模板请参考 [`.env.example`](.env.example)。

### 核心配置

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `8090` | HTTP API 监听端口 |
| `DB_PATH` | `data/clawmem.db` | SQLite 数据库路径（原始文本 + Embedding 缓存） |
| `VECTOR_DB_PATH` | `data/vectors` | Chromem-go 向量索引目录 |

### Embedding 策略

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `EMBEDDING_STRATEGY` | `cloud_first` | Embedding 模型选择策略 |

可选策略：

| 策略 | 行为 |
| :--- | :--- |
| `cloud_first` | Cloudflare → 本地兜底 **（推荐）** |
| `accuracy_first` | OpenAI → Cloudflare → 本地 |
| `local_only` | 仅使用本地模型，绝不调用外部 API |

### 服务商配置

| 变量名 | 说明 |
| :--- | :--- |
| `CF_ACCOUNT_ID` | Cloudflare Account ID（在 Workers & Pages 概览页获取） |
| `CF_API_TOKEN` | Cloudflare API Token（需要 `Workers AI Read` 权限） |
| `EMBED_API_BASE` | *(可选)* OpenAI 兼容的 Embedding 接口地址 |
| `EMBED_API_KEY` | *(可选)* 对应的 API Key |

### LLM 配置（可选）

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `LLM_API_BASE` | — | LLM API 地址（用于记忆摘要） |
| `LLM_API_KEY` | — | LLM API 密钥 |
| `LLM_MODEL` | `gpt-4o-mini` | 模型名称 |
| `DISABLE_LLM_SUMMARY` | `true` | 设为 `false` 启用 LLM 记忆摘要功能 |

### 🌙 梦境引擎 (记忆整合)

Dream (梦境) 是一个可选的后台进程，它会定期将碎片化的记忆整合成简洁、高质量的条目 —— 就像人类大脑在睡眠中整理记忆一样。

**默认禁用。** 设置 `DREAM_ENABLED=true` 开启。禁用时对现有功能零性能损耗。

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `DREAM_ENABLED` | `false` | 是否启用 Dream 记忆整合功能。 |
| `DREAM_INTERVAL` | `24h` | 执行周期（如 `12h`、`6h`、`24h`）。 |
| `DREAM_WINDOW` | `24h` | 每次处理多久以内的原始记忆。 |
| `DREAM_MIN_COUNT` | `10` | 触发整合的最少记忆条数（太少不执行，省 Token）。 |
| `DREAM_MAX_ITEMS` | `200` | 单次处理的最大记忆数（防 Token 爆炸）。 |
| `DREAM_LLM_BASE` | *(同 LLM_API_BASE)* | 可独立配置一个更便宜的模型用作后台整合。 |
| `DREAM_LLM_KEY` | *(同 LLM_API_KEY)* | 对应的 API 密钥。 |
| `DREAM_LLM_MODEL` | *(同 LLM_MODEL)* | 对应的模型名称 (如 `gemini-2.0-flash`)。 |
| `DREAM_PROMPT` | *(内置)* | 自定义系统提示词。 |

#### Dream 是如何工作的

```
┌─────────────────────────────────────────────────────┐
│  每隔 DREAM_INTERVAL (如 24小时)                    │
│                                                     │
│  1. 拉取过去 DREAM_WINDOW 内的活跃记忆碎片          │
│  2. 若条数 < DREAM_MIN_COUNT，跳过                  │
│  3. 发送给 LLM："请整合并提炼这些记忆"              │
│  4. LLM 返回无冲突、简洁的事实清单                  │
│  5. 存入新的“精华记忆”（带特定 Tag，可检索）       │
│  6. 将原始碎片标记为“已整合”（软删除/归档）         │
└─────────────────────────────────────────────────────┘
```

**解决的核心痛点：**
- **记忆冲突**：如果昨天说“我喜欢A”，今天说“我讨厌A”，Dream 会保留最新偏好并解决冲突。
- **信息噪音**：将 500 条闲聊记录浓缩为 5 条高质量事实，大幅提升检索质量。
- **Token 浪费**：更少、更干净的记忆 = 每次检索消耗的 Token 更少，大模型回复更准。

#### 手动触发

您可以通过 API 随时手动触发一次 Dream 周期：

```bash
curl -X POST http://localhost:8090/api/v1/dream/trigger
```

---

## 📡 API 接口

### 存储 / 覆盖记忆

```bash
# 自动智能去重与覆盖 (推荐智能体使用)
curl -X POST http://localhost:8090/api/v1/memo/set \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "kind": "fact",
    "content": "服务器 IP 地址是 192.168.1.100"
  }'

# 简单的添加记忆
curl -X POST http://localhost:8090/api/v1/memo \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "kind": "conversation",
    "content": "我想学习如何用 Go 写后端"
  }'
```

### 搜索记忆

```bash
# 搜索 user-001 的最相关记忆，优先返回偏好等高优内容
curl "http://localhost:8090/api/v1/memo/search?user_id=user-001&query=服务器IP&top_k=5"
```

### 软删除记忆

```bash
# 单条删除
curl -X DELETE "http://localhost:8090/api/v1/memo/{id}"

# 按语义条件批量软删
curl -X POST http://localhost:8090/api/v1/memo/delete-by-query \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "query": "旧业务逻辑废弃"
  }'
```

### 健康检查

```bash
curl http://localhost:8090/health
```

---

## 🔌 集成方式

### MCP Server（Claude Desktop / OpenClaw）

ClawMem 内置了 MCP Server 二进制（`clawmem-mcp`），可与所有 MCP 兼容客户端集成。

```json
{
  "mcpServers": {
    "clawmem": {
      "command": "/path/to/clawmem-mcp",
      "args": [],
      "env": {
        "CLAWMEM_URL": "http://localhost:8090"
      }
    }
  }
}
```

### OpenClaw Skill 模式

1. 将 `skills/clawmem` 文件夹复制到 OpenClaw 的技能目录。
2. 安装依赖：`pip install requests`。
3. 完成！Agent 现在可以说：*「帮我记住服务器 IP 是 1.2.3.4」* → 自动通过 ClawMem 存储。

---

## 🗺️ 路线图

- [x] 多级 Embedding 自动降级
- [x] SQLite 语义缓存 + 部分命中差量计算
- [x] 梦境引擎 (后台归档) 与 自愈神医 (向量抢救)
- **[x] v0.3 分层记忆体系与自动多路召回 (Fact/Preference/Summary)**
- **[x] v0.3 智能记忆重写覆写 (Set API) 与并发隔离**
- **[x] v0.3 彻底的多租户与会话物理级读写隔离 (基于 `user_id`)**
- **[x] v0.3 自动衰减、超长生命周期保护与存储预算自动化管理**
- [ ] ONNX Runtime 集成（Int8 量化本地推理）


---

## 📄 许可证

本项目基于 [MIT License](LICENSE) 开源。

---

## 🙏 致谢

本项目**参考并借鉴了 [MemOS](https://github.com/MemTensor/MemOS)** 的架构设计 — 一个非常优秀的 LLM 记忆操作系统。

ClawMem 是基于 **MemOS 设计思想**的轻量化实现与适配，专为 **OpenClaw 智能体生态**定制。

感谢 **MemTensor 团队**的杰出工作。🫡
