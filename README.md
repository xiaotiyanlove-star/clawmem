# ClawMem 🦞

**The "Sovereign Memory" for Low-Cost AI Agents.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/xiaotiyanlove-star/clawmem)](https://goreportcard.com/report/github.com/xiaotiyanlove-star/clawmem)

[🇨🇳 中文文档 (Chinese Documentation)](docs/README_zh.md)

---

### 💡 Why ClawMem?

Running a smart AI Agent usually requires a **Vector Database** and an **Embedding Model**. But for personal agents running on **cheap VPS ($5/mo)**, this is a nightmare:

*   ❌ **Heavy**: Docker containers and Python-based vector DBs eat up RAM (500MB+).
*   ❌ **Slow**: Running local embedding models on a weak CPU makes the agent unresponsive.
*   ❌ **Complex**: You spend more time managing infrastructure than building your agent.

**ClawMem** is designed to be the **lightest, most resilient memory layer** for your sovereign AI agent.

### ✨ Key Benefits

1.  **💸 Token Efficiency (Huge Cost Saver)**:
    *   Without Memory: You must feed the *entire* chat history to the LLM context window. Expensive & slow.
    *   **With ClawMem**: Retrieve only the *top 3 relevant memories*. Keep your context window small, fast, and cheap.
2.  **💰 Zero Infra Cost**: Use **Cloudflare Workers AI (Free Tier)** for GPT-4 level semantic understanding.
3.  **🪶 Featherlight**: Pure Go. No Docker/Python. Binary is **~15MB**, RAM usage **<20MB**.
4.  **🛡️ Bulletproof Resilience**:
    *   **Cloud Down?** Automatically falls back to local models.
    *   **Rate Limit?** Degrades gracefully without crashing.

---

## 🏗️ Architecture

```mermaid
graph TD
    User[OpenClaw Agent] -->|Store/Search| API[HTTP API :8090]
    API --> Service[Core Service]
    Service -->|Text Data| SQLite[(SQLite DB\nRaw Text)]
    Service -->|Get Vector| Manager[Embedding Manager]
    
    subgraph "Tiered Strategies"
        Manager -->|Tier 1 (Priority)| CF[Cloudflare Workers AI]
        Manager -->|Tier 1 (Alt)| OA[OpenAI Compatible]
        Manager -->|Tier 0 (Fallback)| Local[Local Mock/Lite Model]
    end
    
    Manager -->|Vector Data| VectorDB[(Vector Store\nChromem-go)]
    
    style CF fill:#f9f,stroke:#333
    style VectorDB fill:#bbf,stroke:#333
```

---

## ⚡ One-Click Deployment

If you have `root` access and `go` installed:

```bash
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
sudo ./scripts/install.sh
```

**The script will interactively ask for:**
*   Service Port (Default: `8090`)
*   Database Path (Default: `/var/lib/clawmem/...`)
*   Cloudflare Credentials (Account ID & Token)

It automatically compiles the binary and configures the systemd service.

---

## 🔧 Configuration Reference

Configuration is stored in `/etc/clawmem/config.env`.

### Core Settings
| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8090` | The HTTP port the service listens on. |
| `DB_PATH` | `/var/lib/clawmem/clawmem.db` | SQLite file path for storing raw memory text and metadata. |
| `VECTOR_DB_PATH` | `/var/lib/clawmem/vectors` | Directory path for storing vector indices. |

### Embedding Strategy
| Variable | Default | Description |
| :--- | :--- | :--- |
| `EMBEDDING_STRATEGY` | `cloud_first` | `cloud_first`: Try Cloudflare/OpenAI -> Fallback to Local.<br>`local_only`: Never call external APIs.<br>`accuracy_first`: Try OpenAI -> Cloudflare -> Local. |

### Providers
| Variable | Description |
| :--- | :--- |
| `CF_ACCOUNT_ID` | **Cloudflare Account ID**. Found in Workers & Pages overview. |
| `CF_API_TOKEN` | **Cloudflare API Token**. Must have `Workers AI (Read)` permissions. |
| `EMBED_API_BASE` | (Optional) OpenAI-compatible embedding endpoint. |
| `EMBED_API_KEY` | (Optional) Key for the above endpoint. |

### Advanced
| Variable | Default | Description |
| :--- | :--- | :--- |
| `DISABLE_LLM_SUMMARY` | `true` | If `false`, uses LLM to summarize long memories before storage (requires LLM config). |

### 🌙 Dream (Memory Consolidation)

Dream is an optional background process that periodically consolidates fragmented memories into concise, high-quality entries — like how the human brain organizes memories during sleep.

**Disabled by default.** Set `DREAM_ENABLED=true` to activate. When disabled, Dream has zero impact on existing functionality.

| Variable | Default | Description |
| :--- | :--- | :--- |
| `DREAM_ENABLED` | `false` | Enable the Dream memory consolidation feature. |
| `DREAM_INTERVAL` | `24h` | How often to run consolidation (e.g., `12h`, `6h`, `24h`). |
| `DREAM_WINDOW` | `24h` | Time window to look back for memories to consolidate. |
| `DREAM_MIN_COUNT` | `10` | Minimum number of memories required to trigger a cycle (avoids wasting tokens on quiet days). |
| `DREAM_MAX_ITEMS` | `200` | Maximum memories to process per cycle (prevents token explosion). |
| `DREAM_LLM_BASE` | *(uses `LLM_API_BASE`)* | Override LLM endpoint for Dream (e.g., use a cheaper model). |
| `DREAM_LLM_KEY` | *(uses `LLM_API_KEY`)* | Override LLM API key for Dream. |
| `DREAM_LLM_MODEL` | *(uses `LLM_MODEL`)* | Override LLM model for Dream (e.g., `gemini-2.0-flash`). |
| `DREAM_PROMPT` | *(built-in)* | Custom system prompt for the consolidation LLM call. |

#### How Dream Works

```
┌─────────────────────────────────────────────────────┐
│  Every DREAM_INTERVAL (e.g., 24h)                   │
│                                                     │
│  1. Fetch active memories from last DREAM_WINDOW    │
│  2. Skip if count < DREAM_MIN_COUNT                 │
│  3. Send to LLM: "Consolidate these fragments"      │
│  4. LLM returns concise facts (conflict-resolved)   │
│  5. Store new "dream" memories (tagged, searchable)  │
│  6. Mark originals as "consolidated" (soft archive)  │
└─────────────────────────────────────────────────────┘
```

**What Dream solves:**
- **Memory conflicts**: If you said "I like A" yesterday and "I hate A" today, Dream keeps only the latest preference.
- **Noise accumulation**: 500 chat fragments → 5 concise facts. Dramatically improves retrieval quality.
- **Token waste**: Smaller, cleaner memory = cheaper and more accurate LLM responses.

#### Manual Trigger

You can trigger a Dream cycle at any time via the API:

```bash
curl -X POST http://localhost:8090/api/v1/dream/trigger
```

#### Example

**Before Dream** (raw fragments):
```
[1] "好的，我知道了"
[2] "服务器 IP 是 1.2.3.4"
[3] "嗯嗯"
[4] "把端口改成 8080"
[5] "之前说的 IP 不对，应该是 5.6.7.8"
[6] "收到"
```

**After Dream** (consolidated):
```
- Server IP: 5.6.7.8 (updated from 1.2.3.4)
- Server port: 8080
```

6 fragments → 2 facts. Clean, conflict-resolved, and searchable.

---

## 🔌 OpenClaw Integration

We recommend using the **Skill Mode** to integrate with OpenClaw without modifying the core configuration.

### Setup

1.  Copy the `skills/clawmem` directory to your OpenClaw skills folder.
2.  Install python dependencies: `pip install requests`.

### Usage in Agent

The agent can now use natural language to store and retrieve memories:

*   **Store**: "Remember that the server IP is 1.2.3.4" -> Calls `clawmem add`.
*   **Recall**: "What was the server IP?" -> Calls `clawmem search`.

## 📄 License

MIT License. See [LICENSE](LICENSE) file.
