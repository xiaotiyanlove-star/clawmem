# ClawMem 使用指南 🦞

**低成本 AI Agent 的“主权记忆层”。**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

### 💡 为什么需要 ClawMem?

通常，要让 AI Agent 拥有长期记忆，你需要部署一个 **向量数据库** 和一个 **Embedding 模型**。但对于运行在 **廉价 VPS ($5/月)** 上的个人 Agent 来说，这简直是灾难：

*   ❌ **太重了**: Docker 容器和 Python 向量库非常吃内存 (500MB+)。
*   ❌ **太慢了**: 在弱 CPU 上跑本地模型会让 Agent 反应迟钝。
*   ❌ **太麻烦**: 维护基础设施比写代码还累。

**ClawMem** 是专为低配、主权级 AI Agent 设计的**极轻量、高韧性记忆层**。

### ✨ 核心价值 (尤其是省钱！)

1.  **💸 节省 Token (上下文瘦身)**:
    *   **没有记忆**: 你被迫把之前的几万字聊天记录全部塞进 Prompt，既慢又贵。
    *   **有了 ClawMem**: 只需检索最相关的 3 条记忆片段。**大幅减少 Context Window 占用，直接降低 API 账单。**
2.  **💰 零基建成本**: 利用 **Cloudflare Workers AI (免费层)** 获得高质量语义理解。
3.  **🪶 羽量级占用**: 纯 Go 编写。无 Docker/Python。二进制仅 **~15MB**，内存占用 **<20MB**。
4.  **🛡️ 究极稳健**: 即使断网或 API 挂了，服务也不会崩溃，而是自动降级运行。

---

## 🏗️ 架构图

```mermaid
graph TD
    User[OpenClaw Agent] -->|存储/检索| API[HTTP API :8090]
    API --> Service[核心服务]
    Service -->|文本数据| SQLite[(SQLite DB\n原始文本)]
    Service -->|获取向量| Manager[Embedding 管理器]
    
    subgraph "分级策略 (Tiered)"
        Manager -->|Tier 1 (主力)| CF[Cloudflare Workers AI]
        Manager -->|Tier 1 (备选)| OA[OpenAI 兼容接口]
        Manager -->|Tier 0 (兜底)| Local[本地 Mock/轻量模型]
    end
    
    Manager -->|向量数据| VectorDB[(向量库\nChromem-go)]
    
    style CF fill:#f9f,stroke:#333
    style VectorDB fill:#bbf,stroke:#333
```

---

## ⚡ 一键部署 (One-Click)

如果你有 `root` 权限和 `go` 环境，只需运行：

```bash
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
sudo ./scripts/install.sh
```

脚本会交互式询问配置，并自动完成编译和服务启动。

---

## 🔧 配置项详解

配置文件位于 `/etc/clawmem/config.env`。

### 核心设置
| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `8090` | HTTP 服务监听的端口。 |
| `DB_PATH` | `/var/lib/clawmem/clawmem.db` | SQLite 数据库路径，存储记忆原文。 |
| `VECTOR_DB_PATH` | `/var/lib/clawmem/vectors` | 向量数据库索引文件的存储目录。 |

### Embedding 策略
| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `EMBEDDING_STRATEGY` | `cloud_first` | `cloud_first`: 优先用 Cloudflare，失败转本地。<br>`local_only`: 强制只用本地。<br>`accuracy_first`: 优先用 OpenAI (需配置 Key)。 |

### 服务商配置
| 变量名 | 说明 |
| :--- | :--- |
| `CF_ACCOUNT_ID` | **Cloudflare Account ID**。在 Workers & Pages 概览页获取。 |
| `CF_API_TOKEN` | **Cloudflare API Token**。需要有 `Workers AI (Read)` 权限。 |
| `EMBED_API_BASE` | (可选) OpenAI 兼容的 Embedding 接口地址。 |
| `EMBED_API_KEY` | (可选) 对应的 API Key。 |

### 高级选项
| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `DISABLE_LLM_SUMMARY` | `true` | 是否禁用 LLM 自动摘要。开启需要配置 `LLM_*` 相关变量，会增加 Token 消耗但提升记忆质量。 |

### 🌙 Dream（记忆整合）

Dream 是一个可选的后台进程，定期将碎片化的记忆整合为简洁、高质量的条目——就像人类大脑在睡眠时整理记忆一样。

**默认关闭。** 设置 `DREAM_ENABLED=true` 即可启用。关闭时对现有功能零影响。

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `DREAM_ENABLED` | `false` | 是否启用 Dream 记忆整合功能。 |
| `DREAM_INTERVAL` | `24h` | 整合执行间隔（如 `12h`、`6h`、`24h`）。 |
| `DREAM_WINDOW` | `24h` | 回溯时间窗口，整合多久以内的记忆。 |
| `DREAM_MIN_COUNT` | `10` | 触发整合的最小记忆条数（低于此数跳过，避免浪费 Token）。 |
| `DREAM_MAX_ITEMS` | `200` | 单次最大处理记忆条数（防止 Token 爆炸）。 |
| `DREAM_LLM_BASE` | *(复用 `LLM_API_BASE`)* | Dream 专用 LLM 地址（可用更便宜的模型）。 |
| `DREAM_LLM_KEY` | *(复用 `LLM_API_KEY`)* | Dream 专用 LLM API Key。 |
| `DREAM_LLM_MODEL` | *(复用 `LLM_MODEL`)* | Dream 专用模型名（如 `gemini-2.0-flash`）。 |
| `DREAM_PROMPT` | *(内置默认)* | 自定义整合 Prompt。 |

#### Dream 工作流程

```
┌─────────────────────────────────────────────────────┐
│  每隔 DREAM_INTERVAL（如 24h）自动执行               │
│                                                     │
│  1. 拉取 DREAM_WINDOW 内的活跃记忆                   │
│  2. 数量 < DREAM_MIN_COUNT 则跳过                    │
│  3. 发送给 LLM："请整合这些碎片记忆"                  │
│  4. LLM 返回精炼事实（自动消解冲突）                  │
│  5. 存入新的 "dream" 记忆（带标签，可检索）           │
│  6. 原始记忆标记为 "consolidated"（软归档）           │
└─────────────────────────────────────────────────────┘
```

**Dream 解决的三大痛点：**
- **记忆冲突**：昨天说"喜欢 A"，今天说"讨厌 A"，Dream 只保留最新偏好并记录变更。
- **噪音累积**：500 条闲聊碎片 → 5 条核心事实。检索质量大幅提升。
- **Token 浪费**：更小、更干净的记忆 = 更便宜、更准确的 LLM 响应。

#### 手动触发

随时可通过 API 手动触发一次整合：

```bash
curl -X POST http://localhost:8090/api/v1/dream/trigger
```

#### 效果示例

**整合前**（原始碎片）：
```
[1] "好的，我知道了"
[2] "服务器 IP 是 1.2.3.4"
[3] "嗯嗯"
[4] "把端口改成 8080"
[5] "之前说的 IP 不对，应该是 5.6.7.8"
[6] "收到"
```

**整合后**（精华记忆）：
```
- 服务器 IP: 5.6.7.8（从 1.2.3.4 更新）
- 服务器端口: 8080
```

6 条碎片 → 2 条事实。干净、无冲突、可检索。

---

## 🔌 OpenClaw 接入

最推荐的方式，无需修改 OpenClaw 核心配置。

1.  将 `skills/clawmem` 文件夹复制到你的 OpenClaw 技能目录。
2.  安装依赖: `pip install requests`。
3.  **完成！** 你的 Agent 现在可以说：“帮我记住这个”。

## 📄 许可证

MIT License. 详见 [LICENSE](../LICENSE) 文件。
