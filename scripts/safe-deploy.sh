#!/bin/bash
set -e

# Safe Deploy for PateProject
# Ensures all tests pass before triggering GCP deployment.

echo "================================================"
echo "🛡️  STARTING SAFE DEPLOY PIPELINE"
echo "================================================"

# Low-cost defaults (override in environment if needed)
: "${COST_MODE:=LOW}"
: "${RUN_LLM_SMOKE_TEST:=false}"

# 1. Load Environment
ROOT_ENV="$(dirname "$0")/../.env"
if [ -f "$ROOT_ENV" ]; then
    echo "📋 Loading secrets from $ROOT_ENV..."
    export $(grep -v '^#' "$ROOT_ENV" | grep -v '^$' | xargs)
fi

# Force OpenAI-only mode for tests and deploy.
export LLM_PROVIDER="openai"
export OPENAI_MODEL="${OPENAI_MODEL:-gpt-4o-mini}"

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

# 3. Run Live Smoke Tests (blocking when enabled)
echo ""
if [ "$RUN_LLM_SMOKE_TEST" = "true" ]; then
    echo "🧠 STEP 2: Running Live LLM Smoke Tests..."
    cd backend
    if LLM_SMOKE_TEST=true LLM_PROVIDER=openai OPENAI_MODEL="${OPENAI_MODEL}" go test -v -run TestWhatsAppLLMSmoke ./tests/...; then
        echo "✅ Smoke Tests Passed!"
    else
        echo "❌ Smoke Tests Failed! Aborting deployment."
        exit 1
    fi
    cd ..
else
    echo "🧠 STEP 2: Skipping Live LLM Smoke Tests (RUN_LLM_SMOKE_TEST=false)"
fi

# 4. Trigger Deployment
echo ""
echo "🚀 STEP 3: All tests passed! Triggering GCP Deployment..."
bash scripts/deploy-gcp.sh "$@" --cost-mode "$COST_MODE"

echo ""
echo "🎉 SAFE DEPLOY COMPLETE!"
