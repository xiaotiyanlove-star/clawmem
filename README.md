# ClawMem 🦞

**ClawMem** is a lightweight, tiered memory service designed for OpenClaw agents running on resource-constrained environments (like low-cost VPS).

## 🌟 Key Features

*   **Tiered Embedding Strategy**:
    *   **Tier 1 (Cloud)**: Uses **Cloudflare Workers AI** (Free Tier) or OpenAI for high-performance embeddings.
    *   **Tier 0 (Local Mock/Fallback)**: A lightweight local fallback (Mock implementation in Alpha) ensures the service never crashes even if APIs are down. (Note: Production users should configure valid Cloudflare/OpenAI keys for semantic accuracy).
*   **Lazy Loading**: Local resources are only allocated when absolutely necessary.
*   **Zero CGO**: Built with pure Go libraries (`modernc.org/sqlite`, `chromem-go`), making deployment as simple as copying a single binary. No system dependencies required.
*   **Differential Batching**: Smart caching system that only requests embeddings for new/modified text, saving API costs and time.

## 🚀 Deployment Guide

### 1. Installation

**Option A: Build from Source (Recommended)**

```bash
# Requires Go 1.23+
git clone https://github.com/xiaotiyanlove-star/clawmem
cd clawmem
go build -o clawmem ./cmd/server
sudo mv clawmem /usr/local/bin/
```

**Option B: Download Binary**
Check the [Releases](https://github.com/xiaotiyanlove-star/clawmem/releases) page.

### 2. Configuration (`/etc/clawmem/config.env`)

Create the configuration file.

**Crucial Step for Cloudflare**:
You do **NOT** need to deploy a Worker script. You only need an API Token.
1. Go to Cloudflare Dashboard -> User Profile -> API Tokens.
2. Create Token -> Use template "Workers AI" (Read/Write).
3. Copy the token to `CF_API_TOKEN` below.
4. Get your Account ID from the Workers & Pages overview page.

```bash
PORT=8090
DB_PATH=/var/lib/clawmem/clawmem.db
VECTOR_DB_PATH=/var/lib/clawmem/vectors

# Strategy: cloud_first (Recommended for VPS)
EMBEDDING_STRATEGY=cloud_first

# Cloudflare Configuration
# Account ID: From your Cloudflare URL or Workers dashboard
CF_ACCOUNT_ID=your_account_id
# API Token: Needs "Workers AI" permission
CF_API_TOKEN=your_api_token

# Optional: LLM for summarization
LLM_API_BASE=https://openrouter.ai/api/v1
LLM_API_KEY=
LLM_MODEL=stepfun/step-3.5-flash
DISABLE_LLM_SUMMARY=true
```

### 3. Systemd Service

Create `/etc/systemd/system/clawmem.service`:

```ini
[Unit]
Description=ClawMem Memory Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/clawmem
ExecStart=/usr/local/bin/clawmem
Restart=always
RestartSec=5
EnvironmentFile=/etc/clawmem/config.env

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo mkdir -p /var/lib/clawmem
sudo systemctl enable --now clawmem
```

## 🔌 OpenClaw Integration

In your OpenClaw tool definition (or MCP config), connect to the HTTP API:

*   **Endpoint**: `http://localhost:8090` (or your VPS IP)
*   **API Endpoints**:
    *   `POST /api/v1/memo`: Store a memory.
        ```json
        { "user_id": "user", "content": "I like tech.", "tags": ["preference"] }
        ```
    *   `GET /api/v1/memo/search?q=tech&user_id=user`: Retrieve memories.

## 🛠️ FAQ

**Q: Do I need to deploy a Cloudflare Worker script?**
A: **No.** ClawMem communicates directly with the Cloudflare Workers AI **REST API**. You only need an API Token with the correct permissions.

**Q: What happens if my Cloudflare token is invalid?**
A: ClawMem will detect the failure during its health check or request, mark the provider as "down," and automatically fall back to the Local Tier (currently a Mock/Hash embedding in Alpha to guarantee uptime on low-resource machines).

**Q: Does it support backups?**
A: Yes. All data is stored in `/var/lib/clawmem/clawmem.db`. You can simply copy this file to backup your memories.

**Q: Why "Mock Local Embedder"?**
A: Running a real BERT model locally requires ~200MB-500MB RAM. On a 2.4G VPS running OpenClaw + Docker, this risks OOM. The Alpha version uses a deterministic hash embedding for fallback to ensure the service *never* crashes, even if search accuracy drops during an API outage. For production, please configure a valid Cloudflare/OpenAI key.
