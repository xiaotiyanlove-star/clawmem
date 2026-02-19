import requests
import os
import json

# Default to localhost:8090 if not configured
CLAWMEM_URL = os.getenv("CLAWMEM_URL", "http://localhost:8090")

def add_memory(content: str):
    """
    Store a new memory into ClawMem.
    """
    url = f"{CLAWMEM_URL}/api/memory"
    # In a real agent context, user_id might come from the session or config.
    # Here we default to "default_agent".
    payload = {
        "user_id": "default_agent",
        "content": content
    }
    
    try:
        resp = requests.post(url, json=payload, timeout=5)
        resp.raise_for_status()
        return f"Memory stored successfully. ID: {resp.json().get('id')}"
    except Exception as e:
        return f"Failed to store memory: {str(e)}"

def search_memory(query: str):
    """
    Search for relevant memories in ClawMem.
    """
    url = f"{CLAWMEM_URL}/api/memory/search"
    params = {
        "user_id": "default_agent",
        "q": query,
        "top_k": 3
    }
    
    try:
        resp = requests.get(url, params=params, timeout=5)
        resp.raise_for_status()
        data = resp.json()
        
        if not data:
            return "No relevant memories found."
            
        # Format the results for the agent
        results = []
        for item in data:
            results.append(f"- {item['content']} (Score: {item['similarity']:.2f})")
            
        return "\n".join(results)
    except Exception as e:
        return f"Failed to search memory: {str(e)}"
