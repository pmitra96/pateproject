#!/bin/bash
# Pre-deployment Smoke Test Script

# Ensure we are in the backend directory
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR/.."

echo "🧪 Running Real-LLM Smoke Test..."
echo "--------------------------------"

# Load .env from backend or root
if [ -f ".env" ]; then
    echo "📂 Loading .env from current directory ($(pwd))"
    export $(grep -v '^#' .env | xargs)
elif [ -f "../.env" ]; then
    echo "📂 Loading .env from parent directory"
    export $(grep -v '^#' ../.env | xargs)
else
    echo "⚠️ Warning: No .env file found. Relying on existing environment variables."
fi

# Force OpenAI-only mode
export LLM_PROVIDER="openai"
if [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ Error: OPENAI_API_KEY is not set (OpenAI-only mode)."
    exit 1
fi

# Run only the live smoke test in the tests package
# Setting LLM_SMOKE_TEST=true activates the live API calls
LLM_SMOKE_TEST=true go test -v -run TestWhatsAppLLMSmoke ./tests/...

RESULT=$?

if [ $RESULT -eq 0 ]; then
  echo "--------------------------------"
  echo "✅ Smoke test passed! System is ready for deployment."
  exit 0
else
  echo "--------------------------------"
  echo "❌ Smoke test failed! Check the logs above for LLM/API errors."
  exit 1
fi
