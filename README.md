# ClawMem 🦞

**The "Sovereign Memory" for Low-Cost AI Agents.**

[🇨🇳 中文文档 (Chinese Documentation)](docs/README_zh.md)

---

### 💡 Why ClawMem?

Running a smart AI Agent usually requires a **Vector Database** and an **Embedding Model**. But for personal agents running on **cheap VPS ($5/mo)**, this is a nightmare:

*   ❌ **Heavy**: Docker containers and Python-based vector DBs eat up RAM (500MB+).
*   ❌ **Slow**: Running local embedding models on a weak CPU makes the agent unresponsive.
*   ❌ **Complex**: You spend more time managing infrastructure than building your agent.

**ClawMem** is designed to be the **lightest, most resilient memory layer** for your sovereign AI agent.

### 🧠 The "Magic" of Vectors (Why you need this)

Traditional databases use **Keyword Search**.
*   *You search*: "Apple" -> *Result*: "Apple pie" (Matches word).
*   *You search*: "iPhone" -> *Result*: Nothing (No match).

ClawMem uses **Vector Semantic Search**.
*   It converts text into numbers (vectors) representing **meaning**.
*   *You search*: "Fruit" -> *Result*: "Apple pie", "Banana" (It understands categories).
*   *You search*: "Device" -> *Result*: "iPhone", "Laptop" (It understands context).

**Benefit**: Your agent stops being a goldfish. It remembers context, preferences, and details naturally, just like a human.

### ✨ Key Benefits

1.  **💰 Zero Cost**: Use **Cloudflare Workers AI (Free Tier)** for GPT-4 level semantic understanding.
2.  **🪶 Featherlight**: Pure Go. No Docker/Python. Binary is **~15MB**, RAM usage **<20MB**.
3.  **🛡️ Bulletproof**:
    *   **Cloud Down?** Falls back to local/mock models.
    *   **Rate Limit?** Degrades gracefully without crashing.
4.  **🧠 Plug-and-Play**: Comes with a ready-to-use **OpenClaw Skill**.

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

Then it auto-compiles and starts the systemd service.

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

### Edit Config
```bash
nano /etc/clawmem/config.env
```
