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

---

## 🔌 OpenClaw 接入

最推荐的方式，无需修改 OpenClaw 核心配置。

1.  将 `skills/clawmem` 文件夹复制到你的 OpenClaw 技能目录。
2.  安装依赖: `pip install requests`。
3.  **完成！** 你的 Agent 现在可以说：“帮我记住这个”。

## 📄 许可证

MIT License. 详见 [LICENSE](../LICENSE) 文件。
