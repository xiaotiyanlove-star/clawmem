/**
 * ClawMem OpenClaw Integration Plugin
 *
 * 使用 clawmem REST API 实现自动记忆存储和召回。
 * 配置参考: ../CLawMem-OpenClaw集成方案.md
 */

const PLUGIN_ID = "clawmem-integration";
const PLUGIN_TAG = `[${PLUGIN_ID}]`;

function truncate(text: string, maxLen: number): string {
  if (!text || !maxLen) return text;
  return text.length > maxLen ? `${text.slice(0, maxLen)}...` : text;
}

function extractText(content: any): string {
  if (!content) return "";
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content.filter((b: any) => b?.type === "text").map((b: any) => b.text).join(" ");
  }
  return "";
}

function pad2(v: number): string {
  return String(v).padStart(2, "0");
}

function formatTime(ts?: number | string): string {
  const d = ts ? new Date(ts) : new Date();
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
}

function buildPrependContext({ qmdResults, conversations, preferences, nowText }: any): string {
  const lines = [];
  lines.push("# Role", "");
  lines.push("You are an intelligent assistant with long-term memory. Use the retrieved memory fragments below to provide personalized, accurate responses.", "");
  lines.push("# System Context", "");
  lines.push(`- Current Time: ${nowText}`, "");
  lines.push("# Memory Data", "");

  if (qmdResults?.length) {
    lines.push("## Long-term Memory (from ClawMem knowledge base)", "```text", "<facts>");
    for (const item of qmdResults) {
      const snippet = item.snippet || item.content || "";
      if (snippet) lines.push(` - ${snippet.replace(/\n/g, " ").trim()}`);
    }
    lines.push("</facts>", "```", "");
  }

  if (conversations?.length) {
    lines.push("## Recent Conversations", "```text", "<recent_context>");
    for (const item of conversations) {
      const time = item.created_at || "";
      if (item.source === "summary") {
        lines.push(` - [${time}] (summary) ${item.content}`);
      } else {
        lines.push(` - [${time}] [${item.role}] ${truncate(item.content, 500)}`);
      }
    }
    lines.push("</recent_context>", "```", "");
  }

  if (preferences?.length) {
    lines.push("## User Preferences", "```text", "<preferences>");
    for (const pref of preferences) {
      const typeLabel = pref.type === "implicit" ? "[Implicit]" : "[Explicit]";
      lines.push(` - ${typeLabel} ${pref.preference}`);
    }
    lines.push("</preferences>", "```", "");
  }

  lines.push("# Memory Safety Protocol", "");
  lines.push("Before using any memory above, apply these checks:");
  lines.push("1. Source: Is this a direct user statement or AI inference?");
  lines.push("2. Attribution: Is the subject definitely the user?");
  lines.push("3. Relevance: Does this directly help answer the current query?");
  lines.push("4. Freshness: If memory conflicts with current intent, prioritize the current query.", "");
  lines.push("# Attention", "");
  lines.push("Relevant memory context is already provided above. Do NOT read from or write to local MEMORY.md or memory/* files for reference — they may be outdated or redundant with the injected context. Focus on the user's current query.", "");

  return lines.join("\n");
}

/**
 * 插件入口
 */
export default {
  id: PLUGIN_ID,
  name: "ClawMem Integration",
  description: "Auto-capture conversations and recall relevant memories via ClawMem REST API.",
  kind: "lifecycle",
  configSchema: {
    type: "object",
    properties: {
      baseUrl: {
        type: "string",
        description: "ClawMem API base URL, e.g. https://clawmem.example.com/api/v1 or http://localhost:8090/api/v1"
      },
      authToken: {
        type: "string",
        description: "Bearer token for ClawMem API (AUTH_TOKEN)"
      },
      defaultUser: {
        type: "string",
        default: "default",
        description: "User ID to use for memory storage/recall"
      },
      memoryLimit: {
        type: "integer",
        default: 6,
        description: "Number of memories to recall"
      },
      storeEnabled: {
        type: "boolean",
        default: true,
        description: "Enable auto-storage after conversation"
      },
      recallEnabled: {
        type: "boolean",
        default: true,
        description: "Enable recall before conversation"
      },
      maxMessageChars: {
        type: "integer",
        default: 20000,
        description: "Truncate each message to this length"
      }
    },
    required: ["baseUrl", "authToken"]
  },

  register(api: any) {
    const cfg = api.pluginConfig || {};
    const log = api.logger || console;
    const baseUrl = cfg.baseUrl?.replace(/\/+$/, '').trim();
    const authToken = cfg.authToken?.trim();

    if (!baseUrl || !authToken) {
      log.warn?.(`${PLUGIN_TAG} Missing baseUrl or authToken, plugin disabled`);
      return;
    }

    const headers = {
      "Authorization": `Bearer ${authToken}`,
      "Content-Type": "application/json"
    };

    const isAgentAllowed = (ctx: any): boolean => {
      const agentIds = cfg.agentIds;
      if (!agentIds || !Array.isArray(agentIds) || agentIds.length === 0) return true;
      return agentIds.includes(ctx?.agentId);
    };

    // before_agent_start: 从 ClawMem 召回记忆
    api.on("before_agent_start", async (event: any, ctx: any) => {
      if (!cfg.recallEnabled || !isAgentAllowed(ctx)) return;
      const prompt = typeof event?.prompt === "string" ? event.prompt : "";
      if (!prompt || prompt.length < 3) return;

      try {
        const agentId = ctx?.agentId || "unknown";
        const qmdResults = [];

        // 优先使用 OpenClaw 自带的 QMD（如果存在）
        try {
          if (api.runtime?.tools?.memory_search) {
            const r = await api.runtime.tools.memory_search({
              query: prompt,
              limit: cfg.memoryLimit
            });
            if (r?.results) qmdResults.push(...r.results);
          }
        } catch (e) {
          log.warn?.(`${PLUGIN_TAG} QMD search failed: ${e}`);
        }

        // 如果 QMD 不足，再从 clawmem 搜索
        let clawmemResults = [];
        if (qmdResults.length < cfg.memoryLimit) {
          try {
            const queryUrl = new URL(`${baseUrl}/api/v1/memo/search`);
            queryUrl.searchParams.append("user_id", cfg.defaultUser);
            queryUrl.searchParams.append("query", prompt);
            queryUrl.searchParams.append("top_k", String(cfg.memoryLimit - qmdResults.length));

            const resp = await fetch(queryUrl.toString(), {
              method: 'GET',
              headers,
              signal: AbortSignal.timeout(5000)
            });
            if (resp.ok) {
              const { data } = await resp.json();
              clawmemResults = data || [];
            }
          } catch (e) {
            log.warn?.(`${PLUGIN_TAG} ClawMem search failed: ${e}`);
          }
        }

        // 合并结果
        const allResults = [...qmdResults, ...clawmemResults];
        if (!allResults.length) return;

        // 转换为统一格式
        const memories = allResults.map((item: any) => {
          const mem = item?.memory || item;
          return {
            content: mem.content,
            created_at: mem.created_at,
            source: 'clawmem'
          };
        });

        const nowText = formatTime();
        const prepend = buildPrependContext({
          qmdResults: memories,
          conversations: [],
          preferences: [],
          nowText
        });

        return { prependContext: prepend };
      } catch (err) {
        log.warn?.(`${PLUGIN_TAG} recall error: ${err}`);
      }
    });

    // agent_end: 存储对话到 ClawMem
    api.on("agent_end", async (event: any, ctx: any) => {
      if (!cfg.storeEnabled || !isAgentAllowed(ctx)) return;
      if (!event?.success || !event?.messages?.length) return;

      try {
        const agentId = ctx?.agentId || "unknown";
        const sessionId = ctx?.sessionKey || ctx?.sessionId || "";
        const messages = (() => {
          const msgs = [];
          for (const msg of event.messages) {
            const role = msg?.role;
            if (!role || role === "system") continue;
            const content = extractText(msg?.content);
            if (!content) continue;
            msgs.push({ role, content: truncate(content, cfg.maxMessageChars) });
          }
          return msgs;
        })();

        if (!messages.length) return;

        const conversation = messages.map((m: any) => `[${m.role}] ${m.content}`).join("\n");
        const payload = {
          user_id: cfg.defaultUser,
          content: conversation,
          tags: ["openclaw", `session:${sessionId}`, `agent:${agentId}`]
        };

        // 使用 /memo/set 进行智能去重
        await fetch(`${baseUrl}/api/v1/memo/set`, {
          method: 'POST',
          headers,
          body: JSON.stringify(payload),
          signal: AbortSignal.timeout(5000)
        });
        log.info?.(`${PLUGIN_TAG} Stored conversation (${conversation.length} chars)`);
      } catch (err) {
        log.warn?.(`${PLUGIN_TAG} storage failed: ${err}`);
      }
    });

    log.info?.(`${PLUGIN_TAG} Initialized (baseUrl=${baseUrl})`);
  }
};