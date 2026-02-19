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

## 🌟 Key Features

*   **Tiered Embedding Strategy**:
    *   **Tier 1 (Cloud)**: Uses **Cloudflare Workers AI** (Free Tier) or OpenAI for high-performance embeddings.
    *   **Tier 0 (Local Mock/Fallback)**: A lightweight local fallback ensures the service never crashes even if APIs are down.
*   **Lazy Loading**: Local resources are only allocated when absolutely necessary.
*   **Zero CGO**: Built with pure Go libraries (`modernc.org/sqlite`, `chromem-go`), making deployment as simple as copying a single binary.
*   **Differential Batching**: Smart caching system that only requests embeddings for new/modified text.

## 🚀 Deployment Guide

### 1. Installation

**Option A: Build from Source**

```bash
# Requires Go 1.23+
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
go build -o clawmem ./cmd/server
sudo mv clawmem /usr/local/bin/
```

### 2. Configuration

Create `/etc/clawmem/config.env`:

```bash
PORT=8090
DB_PATH=/var/lib/clawmem/clawmem.db
VECTOR_DB_PATH=/var/lib/clawmem/vectors

# Strategy: cloud_first (Recommended)
EMBEDDING_STRATEGY=cloud_first

# Cloudflare Configuration (Free Tier - Workers AI)
# Get Account ID & API Token (Template: Workers AI) from Cloudflare Dashboard
CF_ACCOUNT_ID=your_account_id
CF_API_TOKEN=your_api_token

# Optional: LLM for summarization
DISABLE_LLM_SUMMARY=true
```

### 3. Run as Service

```bash
# Copy systemd file
sudo cp deployment/clawmem.service /etc/systemd/system/
sudo mkdir -p /var/lib/clawmem
sudo systemctl enable --now clawmem
```

*(Note: If `deployment/clawmem.service` is missing, refer to the [Chinese Docs](docs/README_zh.md) for the full content)*

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

## 🛠️ FAQ

**Q: Do I need to deploy a Cloudflare Worker script?**
A: **No.** ClawMem uses the standard Cloudflare Workers AI REST API. You only need a valid API Token.

**Q: Why Mock Embedder?**
A: To prevent OOM on 2GB RAM servers when external APIs fail. Production users should ensure Cloudflare/OpenAI keys are valid.
