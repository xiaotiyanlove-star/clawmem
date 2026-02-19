# ClawMem 使用指南 🦞

**低成本 AI Agent 的“主权记忆层”。**

---

### 💡 为什么需要 ClawMem?

通常，要让 AI Agent 拥有长期记忆，你需要部署一个 **向量数据库** 和一个 **Embedding 模型**。但对于运行在 **廉价 VPS ($5/月)** 上的个人 Agent 来说，这简直是灾难：

*   ❌ **太重了**: Docker 容器和 Python 向量库非常吃内存 (500MB+)。
*   ❌ **太慢了**: 在弱 CPU 上跑本地模型会让 Agent 反应迟钝。
*   ❌ **太麻烦**: 维护基础设施比写代码还累。

**ClawMem** 是专为低配、主权级 AI Agent 设计的**极轻量、高韧性记忆层**。

### 🧠 向量数据库的“魔法” (核心价值)

传统数据库使用 **关键词匹配**：
*   *你搜*: "苹果" -> *结果*: "苹果派" (匹配字面)。
*   *你搜*: "手机" -> *结果*: 无 (因为没匹配到字)。

ClawMem 使用 **向量语义搜索**：
*   它把文字转化成代表**含义**的数字坐标。
*   *你搜*: "水果" -> *结果*: "苹果", "香蕉" (它懂分类)。
*   *你搜*: "数码产品" -> *结果*: "手机", "笔记本" (它懂语境)。

**好处**：你的 Agent 不再是只有7秒记忆的金鱼。它能像人一样自然地联想、回忆上下文和偏好。

### ✨ 核心优势

1.  **💰 零成本**: 利用 **Cloudflare Workers AI (免费层)** 获得高质量语义理解。
2.  **🪶 羽量级**: 纯 Go 编写。无 Docker/Python。二进制仅 **~15MB**，内存占用 **<20MB**。
3.  **🛡️ 究极稳健**:
    *   **断网/限流?** 自动优雅降级，绝不让 Agent 崩溃。
4.  **🧠 开箱即用**: 内置 OpenClaw **Skill**，复制即用。

---

## ⚡ 一键部署 (One-Click)

如果你有 `root` 权限和 `go` 环境，只需运行：

```bash
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
sudo ./scripts/install.sh
```

**脚本会交互式询问配置 (支持回车使用默认值):**
*   服务端口 (默认: `8090`)
*   数据库路径 (默认: `/var/lib/clawmem/...`)
*   Cloudflare 账户信息 (Account ID & Token)

脚本会自动完成编译、配置和服务启动。

---

## 🔌 OpenClaw 接入 (技能模式)

最推荐的方式，无需修改 OpenClaw 核心配置。

1.  将 `skills/clawmem` 文件夹复制到你的 OpenClaw 技能目录。
2.  安装依赖: `pip install requests`。
3.  **完成！** 你的 Agent 现在可以说：“帮我记住这个”。

## 🛠️ 运维备忘录

### 查看状态
```bash
systemctl status clawmem
```

### 查看日志
```bash
journalctl -u clawmem -f
```

### 重启服务
```bash
systemctl restart clawmem
```

### 修改配置
```bash
nano /etc/clawmem/config.env
```
