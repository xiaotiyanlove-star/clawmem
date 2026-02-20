import requests
import os
import json

# Default to localhost:8090 if not configured
CLAWMEM_URL = os.getenv("CLAWMEM_URL", "http://localhost:8090")
# Unified default user_id to match MCP
DEFAULT_USER_ID = os.getenv("CLAWMEM_USER_ID", "global_user")

def add_memory(content: str, user_id: str = None):
    """
    Store a new memory into ClawMem.
    :param content: The content to remember.
    :param user_id: (Optional) The unique user ID or session ID.
    """
    if not content or not content.strip():
        return "Failed to store memory: content cannot be empty."

    url = f"{CLAWMEM_URL}/api/v1/memo"
    payload = {
        "user_id": user_id or DEFAULT_USER_ID,
        "content": content
    }
    
    try:
        resp = requests.post(url, json=payload, timeout=10)
        resp.raise_for_status()
        return f"Memory stored successfully. ID: {resp.json().get('data', {}).get('id')}"
    except requests.exceptions.HTTPError as e:
        if e.response is not None:
             return f"Failed to store memory (HTTP {e.response.status_code}): {e.response.text}"
        return f"Failed to store memory: {str(e)}"
    except Exception as e:
        return f"Failed to store memory: {str(e)}"

def search_memory(query: str, user_id: str = None):
    """
    Search for relevant memories in ClawMem.
    :param query: The search query.
    :param user_id: (Optional) The unique user ID or session ID.
    """
    if not query or not query.strip():
        return "Failed to search memory: query cannot be empty."

    url = f"{CLAWMEM_URL}/api/v1/memo/search"
    params = {
        "user_id": user_id or DEFAULT_USER_ID,
        "query": query,
        "top_k": 5
    }
    
    try:
        resp = requests.get(url, params=params, timeout=10)
        resp.raise_for_status()
        data = resp.json().get('data', [])
        
        if not data:
            return "No relevant memories found."
            
        # Format the results for the agent
        results = []
        for item in data:
            mem = item.get('memory', {})
            results.append(f"- {mem.get('content')} (Score: {item['score']:.2f})")
            
        return "\n".join(results)
    except Exception as e:
        return f"Failed to search memory: {str(e)}"
