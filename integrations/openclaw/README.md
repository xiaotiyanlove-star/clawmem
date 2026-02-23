# ClawMem OpenClaw Integration Plugin

让 OpenClaw 使用 ClawMem 作为长期记忆后端，实现自动存储与智能召回。

## ✨ 功能

- **自动存储**：每次对话结束后自动将对话内容持久化到 ClawMem
- **智能召回**：每次对话前自动搜索相关记忆并注入到 prompt 中
- **零依赖**：仅依赖 OpenClaw 运行时和 ClawMem REST API
- **可配置**：灵活控制启用范围、记忆数量、用户 ID 等

## 📦 快速安装 (3 步)

### 1. 创建插件与配置目录
```bash
mkdir -p ~/.openclaw/extensions/clawmem-integration/lib
mkdir -p ~/.openclaw/extensions/clawmem-integration/config
```

### 2. 复制核心文件
假定您当前在 `clawmem/integrations/openclaw/` 目录下：
```bash
cp plugin.js ~/.openclaw/extensions/clawmem-integration/
cp openclaw.plugin.json ~/.openclaw/extensions/clawmem-integration/
```
*(开发者也可使用 `ln -s` 建立软链接替代 `cp`，以便随时调试更新)*

### 3. 配置 OpenClaw
在您的 `~/.openclaw/openclaw.json` (或独立配置) 中的 `plugins.entries` 追加如下节点：

```json
{
  "plugins": {
    "entries": {
      "clawmem-integration": {
        "enabled": true,
        "config": {
          "baseUrl": "http://127.0.0.1:8080/api/v1",
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

## 🛠️ 故障排除

| 问题现象 | 可能原因 | 解决思路 |
|----------|----------|----------|
| **插件未加载** | 配置文件错位或路径不正确 | 确认 `openclaw.plugin.json` 放置在 `~/.openclaw/extensions/clawmem-integration/` 目录下且 JSON 语法格式正确。 |
| **API 返回 404** | 服务没启动或 `baseUrl` 填错 | 检查 `baseUrl` 最后是否包含了 `/api/v1`，确认 ClawMem 侧进程正在运行。 |
| **API 返回 401** | Token 不匹配或格式错 | 核对 `authToken` 是否与 ClawMem 服务端的 `AUTH_TOKEN` 完全一致，注意头尾不能残留空格。 |
| **未见自动召回** | 开关未开启或搜索不到内容 | 核对 `recallEnabled: true`。若刚搭建，请先进行两轮正常对话积累数据再刷新重试。 |
| **未见自动存储** | 截断保护或网络阻断 | 检查 `storeEnabled: true`，确保配置中的服务器 IP 能被宿机外网访问。 |

## 📝 与 ClawMem Dashboard 的关系

- 本插件仅使用 ClawMem 的 **REST API**
- Dashboard 是独立的管理界面，不影响插件功能
- 建议为 API 和 Dashboard 分别配置不同的 token/password

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