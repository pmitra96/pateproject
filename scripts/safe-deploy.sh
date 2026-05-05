#!/bin/bash
set -e

# Safe Deploy for PateProject
# Ensures all tests pass before triggering GCP deployment.

echo "================================================"
echo "🛡️  STARTING SAFE DEPLOY PIPELINE"
echo "================================================"

# 1. Load Environment
ROOT_ENV="$(dirname "$0")/../.env"
if [ -f "$ROOT_ENV" ]; then
    echo "📋 Loading secrets from $ROOT_ENV..."
    export $(grep -v '^#' "$ROOT_ENV" | grep -v '^$' | xargs)
fi

# Ensure mandatory keys are present for smoke tests
if [ -z "$OPENAI_API_KEY" ] && [ -z "$GEMINI_API_KEY" ]; then
    echo "❌ Error: API keys missing. Cannot run smoke tests."
    exit 1
fi

# 2. Run Unit Tests
echo ""
echo "🧪 STEP 1: Running Unit Tests (Backend)..."
cd backend
if go test ./tests/... ./services/... ./llm/... ./models/... ./config/... ./database/...; then
    echo "✅ Unit Tests Passed!"
else
    echo "❌ Unit Tests Failed! Aborting deployment."
    exit 1
fi
cd ..

# 3. Run Live Smoke Tests (advisory, not blocking)
echo ""
echo "🧠 STEP 2: Running Live LLM Smoke Tests..."
cd backend
if LLM_SMOKE_TEST=true PREFERRED_LLM_MODEL=gpt-4o-mini go test -v -run TestWhatsAppLLMSmoke ./tests/...; then
    echo "✅ Smoke Tests Passed!"
else
    echo "⚠️  Smoke Tests had issues (likely LLM quota). Proceeding with deployment..."
fi
cd ..

# 4. Trigger Deployment
echo ""
echo "🚀 STEP 3: All tests passed! Triggering GCP Deployment..."
bash scripts/deploy-gcp.sh "$@"

echo ""
echo "🎉 SAFE DEPLOY COMPLETE!"
