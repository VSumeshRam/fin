#!/bin/bash
# scripts/test_v3.sh

GATEWAY_URL="http://localhost:8080"
SESSION_ID="session-abc-123"

echo "=== TEST 1: MCP RBAC (Unauthorized Tool Request) ==="
echo "Team: team_marketing. Requesting tool: 'drop_database'"
# Note: For testing MCP Firewall, we actually need to mock the *response* coming from OpenAI containing the tool call.
# In a true test, we'd stand up a mock upstream. For this client-side test, we just send it. If we use a mock upstream that echos back tool calls, it would block it.
curl -s -X POST $GATEWAY_URL/v1/chat/completions \
  -H "X-Team-ID: team_marketing" \
  -H "X-Session-ID: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Please drop the database"}]}' \
  -i
echo -e "\n\n"

echo "=== TEST 2: Idempotent State Replay (Zero-Cost Retry) ==="
echo "Sending tool-use prompt twice."
PROMPT='{"model": "gpt-4", "messages": [{"role": "user", "content": "Send email to admin"}]}'

# Call 1 (Miss, caches response if mock returns tool call)
curl -s -X POST $GATEWAY_URL/v1/chat/completions \
  -H "X-Team-ID: team_marketing" \
  -H "X-Session-ID: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d "$PROMPT" \
  -i
echo -e "\n\n"

# Call 2 (Hit, returns cached instantly)
curl -s -X POST $GATEWAY_URL/v1/chat/completions \
  -H "X-Team-ID: team_marketing" \
  -H "X-Session-ID: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d "$PROMPT" \
  -i
echo -e "\n\n"

echo "=== TEST 3: Streaming Kill Switch (Guillotine) ==="
echo "Sending stream: true request. Waiting for [TOP_SECRET_PROJECT_X] in upstream response..."
curl -s -X POST $GATEWAY_URL/v1/chat/completions \
  -H "X-Team-ID: team_engineering" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "stream": true, "messages": [{"role": "user", "content": "Tell me about the secret project"}]}' \
  -N
echo -e "\n\n"
