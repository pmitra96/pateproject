#!/bin/bash
# Connectivity and LLM test script for NIRA backend
# Tests: local OpenAI, backend health, and simulated webhook

set -e
source "$(dirname "$0")/../.env"

BACKEND_URL="https://pateproject-backend-hgh2pji4tq-uc.a.run.app"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================"
echo "  NIRA Backend Connectivity Test"
echo "========================================"

# --- TEST 1: Backend health ---
echo ""
echo "▶ TEST 1: Backend health check..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BACKEND_URL/health" --max-time 10)
if [ "$STATUS" = "200" ]; then
  echo -e "  ${GREEN}✅ Backend is UP (HTTP $STATUS)${NC}"
else
  echo -e "  ${RED}❌ Backend returned HTTP $STATUS${NC}"
fi

# --- TEST 2: OpenAI from local machine ---
echo ""
echo "▶ TEST 2: OpenAI API reachability (from YOUR machine)..."
RESULT=$(curl -s -o /dev/null -w "%{http_code} %{time_total}s" \
  -X POST https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $LLM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":5}' \
  --max-time 15)
echo -e "  ${GREEN}✅ Result: $RESULT${NC}"

# --- TEST 3: Simulated webhook call to backend (direct LLM trigger) ---
echo ""
echo "▶ TEST 3: Simulated webhook to backend (triggers LLM call)..."
echo "  Sending fake WhatsApp message to backend..."

# Fake but valid-looking webhook payload
PAYLOAD='{
  "object": "whatsapp_business_account",
  "entry": [{
    "changes": [{
      "value": {
        "messages": [{
          "id": "wamid.TEST_CONNECTIVITY_CHECK",
          "from": "918309619180",
          "type": "text",
          "text": {"body": "connectivity test - hey"}
        }]
      }
    }]
  }]
}'

RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code} TIME:%{time_total}s" \
  -X POST "$BACKEND_URL/whatsapp/webhook" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" \
  --max-time 30)

HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | sed 's/.*HTTP_CODE://' | cut -d' ' -f1)
TIME=$(echo "$RESPONSE" | grep "HTTP_CODE:" | sed 's/.*TIME://')

if [ "$HTTP_CODE" = "200" ]; then
  echo -e "  ${GREEN}✅ Backend accepted webhook (HTTP $HTTP_CODE in $TIME)${NC}"
else
  echo -e "  ${YELLOW}⚠️  Backend responded HTTP $HTTP_CODE in $TIME${NC}"
fi

# --- TEST 4: Check LLM usage log for new entry ---
echo ""
echo "▶ TEST 4: Checking DB for new LLM usage log (wait 10s for processing)..."
sleep 10
LATEST=$(psql $DATABASE_URL -t -c "SELECT id, total_tokens, created_at FROM llm_usage_logs ORDER BY created_at DESC LIMIT 1;")
echo "  Latest log entry: $LATEST"

echo ""
echo "========================================"
echo "  Done. Check results above."
echo "========================================"
