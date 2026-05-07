#!/bin/bash
set -e

echo "🧱 Running conversation quality gate..."
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPORT_DIR="${QUALITY_REPORT_DIR:-$ROOT_DIR/artifacts/quality}"
mkdir -p "$REPORT_DIR"
STAMP="$(date +%Y%m%d-%H%M%S)"
LATEST_REPORT="$REPORT_DIR/latest.json"
RUN_REPORT="$REPORT_DIR/quality-$STAMP.json"

cd backend
QUALITY_REPORT_PATH="$LATEST_REPORT" go test ./tests/... -run TestQualityGate -v
cd ..
cp "$LATEST_REPORT" "$RUN_REPORT"
echo "📊 Quality report: $LATEST_REPORT"
echo "🗂️  Snapshot report: $RUN_REPORT"
echo "✅ Quality gate passed."
