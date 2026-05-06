#!/bin/bash
set -e

echo "🧱 Running conversation quality gate..."
cd backend
go test ./tests/... -run TestQualityGate -v
cd ..
echo "✅ Quality gate passed."
