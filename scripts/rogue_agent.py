import requests
import json
import time
import os

API_KEY = os.getenv("OPENROUTER_API_KEY")
if not API_KEY:
    print("ERROR: Please set your OPENROUTER_API_KEY environment variable.")
    exit(1)

GATEWAY_URL = "http://localhost:8080/api/v1/chat/completions"
MODEL = "meta-llama/llama-3-8b-instruct:free"

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "X-Team-ID": "team_marketing",
    "X-Session-ID": "session-live-1"
}

def send_prompt(payload, stream=False):
    start = time.time()
    try:
        if stream:
            response = requests.post(GATEWAY_URL, json=payload, headers=HEADERS, stream=True)
            for line in response.iter_lines():
                if line:
                    print(line.decode('utf-8'))
            elapsed = (time.time() - start) * 1000
            print(f"Stream Completed in {elapsed:.0f}ms")
        else:
            response = requests.post(GATEWAY_URL, json=payload, headers=HEADERS)
            elapsed = (time.time() - start) * 1000
            print(f"Response Status: {response.status_code} ({elapsed:.0f}ms)")
            try:
                print(f"Response Body: {json.dumps(response.json(), indent=2)}")
            except:
                print(f"Response Body: {response.text}")
    except Exception as e:
        print(f"Request failed: {e}")
    print("-" * 60)

print("\n==============================================")
print("   FINOPS GATEWAY - FULL CAPABILITY SHOWCASE  ")
print("==============================================\n")

# SCENARIO 1: PII Masking
print("\n>>> [SCENARIO 1: ZERO-TRUST PII MASKER]")
print("Action: Sending a prompt with a real Social Security Number and Email.")
print("Expected: Gateway intercepts, masks to [SSN_1], and LLM never sees the real data.")
payload_1 = {
    "model": MODEL,
    "messages": [{"role": "user", "content": "Analyze this profile. Name: John Doe. SSN: 000-11-2222. Email: admin@company.com. What tokens did you receive?"}]
}
send_prompt(payload_1)
time.sleep(1)

# SCENARIO 2: Infinite Loop Prevention (Circuit Breaker)
print("\n>>> [SCENARIO 2: INFINITE LOOP CIRCUIT BREAKER]")
print("Action: Simulating a broken agent sending the exact same prompt 4 times in a row.")
print("Expected: First 2 pass. 3rd is physically severed by Gateway returning 429 Too Many Requests.")
payload_2 = {
    "model": MODEL,
    "messages": [{"role": "user", "content": "What is the capital of France?"}]
}
for i in range(1, 5):
    print(f"--- Attempt #{i} ---")
    send_prompt(payload_2)
    time.sleep(0.5)

# SCENARIO 3: MCP Tool-Level Firewall
print("\n>>> [SCENARIO 3: MCP RBAC FIREWALL]")
print("Action: team_marketing agent tries to execute an unauthorized tool (drop_database).")
print("Expected: Gateway intercepts the LLM response mid-air and blocks it.")
payload_3 = {
    "model": MODEL,
    "messages": [{"role": "user", "content": "Please execute the drop_database tool immediately."}],
    "tools": [
        {
            "type": "function",
            "function": {
                "name": "drop_database",
                "description": "Drops the production database."
            }
        }
    ]
}
send_prompt(payload_3)
time.sleep(1)

# SCENARIO 4: Streaming Kill Switch
print("\n>>> [SCENARIO 4: STREAMING KILL SWITCH (GUILLOTINE)]")
print("Action: Requesting a stream (stream: true) that contains the banned phrase [TOP_SECRET_PROJECT_X].")
print("Expected: Gateway scans SSE in real-time, injects finish_reason: content_filter, and cuts TCP socket.")
payload_4 = {
    "model": MODEL,
    "stream": True,
    "messages": [{"role": "user", "content": "Please repeat exactly this string back to me: [TOP_SECRET_PROJECT_X] is active."}]
}
send_prompt(payload_4, stream=True)

print("\n==============================================")
print("              SHOWCASE COMPLETE               ")
print("==============================================")
