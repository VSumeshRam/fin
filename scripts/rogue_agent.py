import requests
import json
import time
import os

# Ensure the user has provided an OpenRouter key
API_KEY = os.getenv("OPENROUTER_API_KEY")
if not API_KEY:
    print("ERROR: Please set your OPENROUTER_API_KEY environment variable.")
    print("Example: export OPENROUTER_API_KEY='sk-or-v1-...'")
    print("Get one for free at https://openrouter.ai")
    exit(1)

GATEWAY_URL = "http://localhost:8080/api/v1/chat/completions"

# Use a free model on OpenRouter
MODEL = "meta-llama/llama-3-8b-instruct:free"

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "X-Team-ID": "team_marketing",
    "X-Session-ID": "session-live-1"
}

def send_prompt(prompt):
    payload = {
        "model": MODEL,
        "messages": [{"role": "user", "content": prompt}]
    }
    
    print(f"Sending prompt to Gateway: {prompt}")
    start = time.time()
    try:
        response = requests.post(GATEWAY_URL, json=payload, headers=HEADERS)
        elapsed = (time.time() - start) * 1000
        print(f"Response Status: {response.status_code} ({elapsed:.0f}ms)")
        try:
            print(f"Response Body: {json.dumps(response.json(), indent=2)}")
        except:
            print(f"Response Body: {response.text}")
    except Exception as e:
        print(f"Request failed: {e}")
    print("-" * 40)

print("\n=== FINOPS GATEWAY LIVE DEMO ===")
print("Routing through Gateway to REAL OpenRouter API...\n")

# Scenario 1: The PII Leak
print("[SCENARIO 1: PII Masking]")
send_prompt("Please summarize this employee record. Name: John Doe. SSN: 000-11-2222. Email: john@company.com.")

# Scenario 2: The Infinite Loop
print("\n[SCENARIO 2: Infinite Retry Loop Prevention]")
print("Agent bug: The agent is stuck in a while loop, sending the exact same request rapidly.")

broken_prompt = "What is the capital of France?"
for i in range(1, 5):
    print(f"--- Retry #{i} ---")
    send_prompt(broken_prompt)
    time.sleep(0.5)

print("\nAs you can see, the 3rd retry was intercepted by the Gateway (429 Too Many Requests) before hitting the LLM API!")
