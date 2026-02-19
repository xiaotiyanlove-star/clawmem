# ClawMem 🦞

**The "Sovereign Memory" for Low-Cost AI Agents.**

[🇨🇳 中文文档 (Chinese Documentation)](docs/README_zh.md)

---

### 💡 Why ClawMem?

Running a smart AI Agent usually requires a **Vector Database** (like Chroma/Qdrant) and an **Embedding Model** (like BERT). But for personal agents running on **cheap VPS ($5/mo, 1-2GB RAM)**, this is a nightmare:

*   ❌ **Heavy**: Docker containers and Python-based vector DBs eat up RAM (500MB+).
*   ❌ **Slow**: Running local embedding models on a weak CPU makes the agent unresponsive.
*   ❌ **Complex**: You spend more time managing infrastructure than building your agent.

**ClawMem is the antidote.** It is designed to be the **lightest, most resilient memory layer** for your sovereign AI agent.

### ✨ What It Gives You

1.  **💰 Zero Cost, High Intelligence**: Use **Cloudflare Workers AI (Free Tier)** to get GPT-4 level semantic understanding without paying a dime or using your VPS CPU.
2.  **🪶 Featherlight Footprint**: Written in pure Go. No Docker, no Python, no CGO. The binary is **~15MB** and idle memory usage is **<20MB**.
3.  **🛡️ Bulletproof Resilience**:
    *   **Cloud Down?** It automatically falls back to a local model (or a deterministic mock on ultra-low-spec hardware).
    *   **API Rate Limit?** It degrades gracefully instead of crashing your agent.
4.  **🧠 "Plug-and-Play" for OpenClaw**: Comes with a ready-to-use **Skill**. Just copy one folder, and your agent can instantly "Remember" and "Recall".

---

## ⚡ One-Click Deployment

If you have `root` access and `go` installed:

```bash
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
sudo ./scripts/install.sh
```

The script will:
1. Build the binary.
2. Ask for your Cloudflare credentials.
3. Configure and start the `systemd` service.

---

## 🔧 Configuration Reference

Configuration is stored in `/etc/clawmem/config.env`.

| Key | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8090` | HTTP Service Port |
| `DB_PATH` | `/var/lib/clawmem/clawmem.db` | Path to SQLite DB (Raw Text) |
| `VECTOR_DB_PATH` | `/var/lib/clawmem/vectors` | Path to Vector DB (Embeddings) |
| `EMBEDDING_STRATEGY` | `cloud_first` | `cloud_first`, `local_only`, or `accuracy_first` |
| `CF_ACCOUNT_ID` | - | Cloudflare Account ID |
| `CF_API_TOKEN` | - | Cloudflare API Token (Requires `Workers AI` permissions) |
| `DISABLE_LLM_SUMMARY` | `true` | Set to `false` to enable LLM summarization (Requires `LLM_*` vars) |

---

## 🔌 OpenClaw Integration (Agent Skills)

We recommend using the **Skill Mode** to integrate with OpenClaw without modifying the core configuration.

### Setup

1.  Copy the `skills/clawmem` directory to your OpenClaw skills folder (e.g., `/root/.openclaw/workspace/skills/`).
2.  Install python dependencies: `pip install requests`.

### Usage in Agent

The agent can now use natural language to store and retrieve memories:

*   **Store**: "Remember that the server IP is 1.2.3.4" -> Calls `clawmem add`.
*   **Recall**: "What was the server IP?" -> Calls `clawmem search`.

See `skills/clawmem/SKILL.md` for details.

## 🛠️ Operations Cheatsheet

### Check Status
```bash
systemctl status clawmem
```

### View Logs
```bash
journalctl -u clawmem -f
```

### Restart Service
(Required after changing config)
```bash
systemctl restart clawmem
```

### Backup Data
All memory data is in a single file. Just copy it:
```bash
cp /var/lib/clawmem/clawmem.db /path/to/backup/
```
