#!/bin/bash
# scripts/test_v2.sh

GATEWAY_URL="http://localhost:8080"
TEAM_ID="team-alpha"

echo "=== TEST 1: Cache Miss + PII Masking ==="
echo "Sending: 'Who is admin@acme.com in 2024?'"
curl -s -X POST $GATEWAY_URL/v1/chat/completions \
  -H "X-Team-ID: $TEAM_ID" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Who is admin@acme.com in 2024?"}], "stream": true}' \
  -i
echo -e "\n\n"

echo "=== TEST 2: Cache Hit ==="
echo "Sending EXACT SAME PROMPT. Should be intercepted by Semantic Cache."
curl -s -X POST $GATEWAY_URL/v1/chat/completions \
  -H "X-Team-ID: $TEAM_ID" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Who is admin@acme.com in 2024?"}], "stream": true}' \
  -i
echo -e "\n\n"

echo "=== TEST 3: Entity Gate Override (Cache Miss) ==="
echo "Sending: 'Who is admin@acme.com in 2025?' (High vector similarity, but Date changed)"
curl -s -X POST $GATEWAY_URL/v1/chat/completions \
  -H "X-Team-ID: $TEAM_ID" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Who is admin@acme.com in 2025?"}], "stream": true}' \
  -i
echo -e "\n\n"
