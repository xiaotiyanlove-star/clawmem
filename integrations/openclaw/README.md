# ClawMem OpenClaw Integration Plugin

让 OpenClaw 使用 ClawMem 作为长期记忆后端，实现自动存储与智能召回。

## ✨ 功能

- **自动存储**：每次对话结束后自动将对话内容持久化到 ClawMem
- **智能召回**：每次对话前自动搜索相关记忆并注入到 prompt 中
- **零依赖**：仅依赖 OpenClaw 运行时和 ClawMem REST API
- **可配置**：灵活控制启用范围、记忆数量、用户 ID 等

## 📦 快速安装 (3 步)

相比于普通的内置 JSON 插件，OpenClaw 需要通过 TypeScript Extensions 目录直接加载我们的生命周期逻辑代码。

### 1. 建立专属存储扩展目录
```bash
mkdir -p ~/.openclaw/extensions/clawmem-integration
```

### 2. 复制核心文件 (或创建软链接)
将本目录下的 `index.ts` 以及 `openclaw.plugin.json` 链接或拷贝过去，**确保文件名一致**：
```bash
# 推荐使用软链接，方便随时同步本地更新
ln -s $(pwd)/index.ts ~/.openclaw/extensions/clawmem-integration/index.ts
ln -s $(pwd)/openclaw.plugin.json ~/.openclaw/extensions/clawmem-integration/openclaw.plugin.json
```

### 3. 配置参数传递 (Plugin Config)
虽然它是通过 TS 文件动态加载的，但它的配置依旧接受 OpenClaw `plugins.entries` 下的参数注入。
在您的 `~/.openclaw/openclaw.json` 里添加：

```json
{
  "plugins": {
    "entries": {
      "clawmem-integration": {
        "enabled": true,
        "config": {
          "baseUrl": "http://127.0.0.1:8090/api/v1",
          "authToken": "CHANGE_ME",
          "defaultUser": "default",
          "memoryLimit": 6,
          "storeEnabled": true,
          "recallEnabled": true,
          "maxMessageChars": 20000,
          "agentIds": []
        }
      }
    }
  }
}
```

### 配置说明

| 字段 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `baseUrl` | string | **必填** | ClawMem API 地址，如 `https://clawmem.example.com/api/v1` |
| `authToken` | string | **必填** | ClawMem 的 `AUTH_TOKEN`，用于 API 鉴权 |
| `defaultUser` | string | `default` | 存储和召回时使用的用户 ID |
| `memoryLimit` | integer | `6` | 每次对话前召回的最近记忆数量 |
| `storeEnabled` | boolean | `true` | 是否在对话结束后自动存储 |
| `recallEnabled` | boolean | `true` | 是否在对话开始前自动召回记忆 |
| `maxMessageChars` | integer | `20000` | 每条消息最大字符数，避免超长存储 |
| `agentIds` | array of string | `[]` | 仅对指定 agent 生效，空数组表示所有 agent |

## 🔄 工作流程

### 对话开始前（before_agent_start）
1. 插件获取用户最新消息作为查询关键词
2. 先调用 OpenClaw 内置 QMD（如有）搜索
3. 再调用 ClawMem API `/api/v1/memo/search` 补充
4. 将合并后的记忆格式化为上下文，注入到 `prependContext`

### 对话结束后（after_agent_end）
1. 提取本轮对话中 user/assistant 的消息
2. 按 `maxMessageChars` 截断每条消息
3. 调用 ClawMem `/api/v1/memo/set` 智能去重存储
4. 标记 tags 包含 `openclaw`, `session:<sessionId>`, `agent:<agentId>`

## 🧪 验证

1. 重启 OpenClaw Gateway:
```bash
openclaw gateway restart
```

2. 查看插件是否加载:
```bash
openclaw plugins list
```
应看到 `clawmem-integration | loaded`

3. 检查日志:
```bash
journalctl -u openclaw -f | grep clawmem-integration
```
期望看到: `[clawmem-integration] Initialized (baseUrl=...)`

4. 发起一次对话，然后检查 ClawMem:
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://clawmem.example.com/api/v1/memo/search?user_id=default&query=你的问题"
```
应该能看到最近对话被存储并召回。

## ⚠️ 避坑指南（重要！）

### 坑 1：入口文件必须是 `.ts`，不能是 `.js`
OpenClaw 的插件加载器扫描路径为 `~/.openclaw/extensions/*/index.ts`。如果你用 `plugin.js` 或普通 JS 文件，**不会被发现加载**。

### 坑 2：`openclaw.plugin.json` 绝对不能删
OpenClaw 强依赖这个文件来验证 `configSchema`。**没有这个文件 = 服务端拒绝注册 = 启动报错。**

### 坑 3：config 字段及 AuthToken 是必填项
如果 `openclaw.json` 中缺少 `baseUrl`/`authToken`，Gateway 启动时会报 `invalid config`，**直接导致所有服务中断并拒绝启动**。
*解法*: 请直接删除损坏配置条目后执行 `openclaw gateway restart` 临时恢复。

### 坑 4：baseUrl 不要带尾部斜杠
❌ 错误: `http://localhost:8090/`
✅ 正确: `http://localhost:8090`
多一个斜杠会导致请求路径拼接成无效的 `//api/v1/...`。

### 坑 5：远程访问需要公网地址
如果 OpenClaw 服务端和 ClawMem 部署在不同机器，请勿使用 `localhost`，需填写能互相触达的公网 IP 或绑定域名。

### 坑 6：插件加载警告 (plugins.allow is empty)
这是 OpenClaw 控制三方注入的新沙盒机制发出的常规提示，属于正常现象。若想消除，可在 `openclaw.json` 中显式添加 `"allow": ["clawmem-integration"]`。

### 坑 7：ClawMem 宕机会阻塞对话吗？
**不会**。本插件的所有网络通信均配置了 `AbortSignal.timeout(5000)` 上界限。如果 ClawMem 不可用，记忆获取或写入会快速超时静默失败，**绝不影响终端用户的聊天响应速度**。

### 坑 8：`openclaw doctor --fix` 的副作用
如果你在缺失 `openclaw.plugin.json` 等残缺状态下允许 doctor 执行自愈，它很可能会把你的 `plugins.entries` 插件挂载声明**直接抹除**。当修复目录结构后，你需要重新在 JSON 添加回来。

---

## 💡 常见问题 (FAQ)

### Q: 如何查看插件是否在工作？
```bash
openclaw logs --follow | grep "\[clawmem\]"
```

### Q: 记忆太多太杂，或是每次抓取太过发散？
1. 在配置中减小 `memoryLimit`（例如改为 3）。
2. 使用 Dashboard 面板主动删除部分时效性弱的信息。
3. ClawMem 后端搭载了 **Dream 梦境引擎**，它能后台异步将大量碎片信息浓缩合并（解决超限问题）。

### Q: 如何只给特定的 Agent 开启自动记忆感知？
```json
"agentIds": ["agent-uuid-string"]  // 指定白名单
```

## 📝 与 ClawMem Dashboard 的关系
- 本插件仅使用 ClawMem 分离暴露的 **REST API**。
- Dashboard 是独立的纯前端管理看板，互不阻塞，强烈建议前后端拆分不同的认证鉴权令牌避免泄露。

## 🤝 后续规划

- 支持从 ClawMem 召回 `summaries` 和 `preferences`（需要 ClawMem API 增加过滤参数）
- 提供批量导出/导入对话的工具
- 支持多用户（OpenClaw 不同 agent 使用不同 `defaultUser`）

## 📄 许可证

MIT（或与原 clawmem 仓库一致）

---

> **兼容性要求**
> - **OpenClaw**: >= 2.14.0
> - **ClawMem**: >= v0.1.0 
> 
> *(依赖包含 `/api/v1/memo/set` 及 `/api/v1/memo/search` 接口的后端支持)*