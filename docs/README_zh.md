# ClawMem 使用指南 🦞

**低成本 AI Agent 的“主权记忆层”。**

[English Documentation](../README.md)

---

### 💡 为什么需要 ClawMem?

通常，要让 AI Agent 拥有长期记忆，你需要部署一个 **向量数据库** (如 Chroma/Qdrant) 和一个 **Embedding 模型** (如 BERT)。但对于运行在 **廉价 VPS ($5/月, 1-2GB 内存)** 上的个人 Agent 来说，这简直是灾难：

*   ❌ **太重了**: Docker 容器和基于 Python 的向量库非常吃内存 (起步 500MB+)。
*   ❌ **太慢了**: 在弱 CPU 上跑本地模型会让 Agent 反应迟钝。
*   ❌ **太麻烦**: 你得花大量时间维护基础设施，而不是构建 Agent 本身。

**ClawMem 就是解药。** 它是专为低配、主权级 AI Agent 设计的**极轻量、高韧性记忆层**。

### ✨ 它能给你带来什么

1.  **💰 零成本，高智商**: 利用 **Cloudflare Workers AI (免费层)**，让你的 Agent 拥有 GPT-4 级别的语义理解能力，既不花钱也不占 VPS CPU。
2.  **🪶 羽量级占用**: 纯 Go 编写。无 Docker，无 Python，无 CGO。二进制文件仅 **~15MB**，空闲内存占用 **<20MB**。
3.  **🛡️ 究极稳健 (永不宕机)**:
    *   **断网了？** 自动降级到本地模型（在极低配机器上甚至可以降级到哈希 Mock 模式）。
    *   **API 限流了？** 优雅降级，而不是直接让 Agent 崩溃报错。
4.  **🧠 OpenClaw 即插即用**: 内置标准 **Skill**。只需复制一个文件夹，你的 Agent 立刻就能学会“记住这个”和“回忆一下”。

---

## ⚡ 一键部署 (One-Click)

如果你有 `root` 权限和 `go` 环境，只需运行：

```bash
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
sudo ./scripts/install.sh
```

脚本会交互式询问你的配置（Cloudflare Token 等），并自动配置 Systemd 服务。

---

## 🔧 配置详解

配置文件位于 `/etc/clawmem/config.env`。

| 配置项 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `8090` | HTTP 服务监听端口 |
| `DB_PATH` | `/var/lib/clawmem/clawmem.db` | SQLite 数据库路径 (存储原始记忆文本) |
| `VECTOR_DB_PATH` | `/var/lib/clawmem/vectors` | 向量数据库路径 (存储 Embedding) |
| `EMBEDDING_STRATEGY` | `cloud_first` | `cloud_first` (优先云端), `local_only` (仅本地), `accuracy_first` (优先 OpenAI) |
| `CF_ACCOUNT_ID` | - | Cloudflare 账户 ID (Dashboard 首页获取) |
| `CF_API_TOKEN` | - | Cloudflare API Token (需有 `Workers AI` 权限) |
| `DISABLE_LLM_SUMMARY` | `true` | 是否禁用 LLM 自动摘要 (开启需配置 LLM_*) |

---

## 🔌 OpenClaw 接入

这是最推荐的接入方式，不需要修改 OpenClaw 核心配置。

1.  将 `skills/clawmem` 文件夹复制到你的 OpenClaw 技能目录。
2.  安装依赖: `pip install requests`。
3.  **完成！** 你的 Agent 现在可以说：“帮我记住这个”。

## 🛠️ 运维备忘录 (Cheatsheet)

### 查看运行状态
```bash
systemctl status clawmem
```

### 查看实时日志
```bash
journalctl -u clawmem -f
```

### 重启服务
(修改配置后需要重启)
```bash
systemctl restart clawmem
```

### 停止服务
```bash
systemctl stop clawmem
```

### 备份数据
所有记忆都在一个文件里，拷贝走即可：
```bash
cp /var/lib/clawmem/clawmem.db /path/to/backup/
```
