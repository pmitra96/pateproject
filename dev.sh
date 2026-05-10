#!/bin/bash
# PateProject Development Server
# Quick start: ./dev.sh

# Function to kill background processes on script exit
cleanup() {
    echo ""
    echo "🛑 Stopping all services..."
    
    # Send SIGTERM to the captured PIDs
    if [ ! -z "$BACKEND_PID" ]; then kill $BACKEND_PID 2>/dev/null; fi
    if [ ! -z "$FRONTEND_PID" ]; then kill $FRONTEND_PID 2>/dev/null; fi
    if [ ! -z "$API_PID" ]; then kill $API_PID 2>/dev/null; fi
    if [ ! -z "$WORKER_PID" ]; then kill $WORKER_PID 2>/dev/null; fi
    
    # Fallback: Kill processes on specific ports
    lsof -i :8080 -t | xargs kill -9 2>/dev/null
    lsof -i :5173 -t | xargs kill -9 2>/dev/null
    lsof -i :8000 -t | xargs kill -9 2>/dev/null
    
    echo "✅ Cleanup complete."
}
trap cleanup EXIT INT TERM

# Load environment variables
if [ -f .env ]; then
  set -a
  source .env
  set +a
fi

# Ensure PostgreSQL binaries are in PATH
export PATH="/opt/homebrew/opt/postgresql@15/bin:$PATH"

echo "🚀 Starting PateProject Development Environment..."

# 1. Check Redis (Required for Celery)
echo "🧠 Checking Redis..."
if ! pgrep redis-server > /dev/null; then
    echo "❌ Redis is not running."
    echo "   Attempting to start local redis..."
    if command -v brew >/dev/null 2>&1; then
       brew services start redis || redis-server --daemonize yes
    else
       echo "   Please start Redis manually."
       exit 1
    fi
fi
echo "✅ Redis is running."

# 2. Check Databases
echo "🐘 Checking Databases..."
# Check Main DB
if ! psql -h localhost -U postgres -lqt | cut -d \| -f 1 | grep -qw pateproject; then
  echo "🛠️ Main DB 'pateproject' not found. Creating..."
  createdb -h localhost -U postgres pateproject
fi
# Check Scraper DB
if ! psql -h localhost -U postgres -lqt | cut -d \| -f 1 | grep -qw scraper_db; then
  echo "🛠️ Scraper DB 'scraper_db' not found. Creating..."
  createdb -h localhost -U postgres scraper_db
fi
echo "✅ Databases are ready!"

# 3. Setup Python Extractor
echo "🐍 Setting up Python Extractor..."
cd python-extractor
if [ ! -d "venv" ]; then
    echo "   Creating venv..."
    python3 -m venv venv
fi
source venv/bin/activate
echo "   Installing dependencies..."
pip install -qr requirements.txt
echo "   Running Migrations..."
alembic upgrade head
echo "   Seeding Categories..."
python -m scripts.seed_categories
cd ..

# 4. Start Backend
echo "📦 Starting Backend (Go)... [Logs: backend.log]"
(cd backend && go run ./cmd/server/main.go) > backend.log 2>&1 &
BACKEND_PID=$!

# 5. Start Python API
echo "🔌 Starting Scraper API (FastAPI)... [Logs: python-api.log]"
(cd python-extractor && source venv/bin/activate && uvicorn app.main:app --host 0.0.0.0 --port 8000 --reload) > python-api.log 2>&1 &
API_PID=$!

# 6. Start Celery Worker
echo "👷 Starting Scraper Worker (Celery)... [Logs: python-worker.log]"
(cd python-extractor && source venv/bin/activate && celery -A app.worker.celery_app worker --loglevel=info -c 20) > python-worker.log 2>&1 &
WORKER_PID=$!

# 7. Start Frontend
echo "🎨 Starting Frontend (React)... [Logs: frontend.log]"
(cd frontend && npm run dev) > frontend.log 2>&1 &
FRONTEND_PID=$!

echo ""
echo "✅ All services started!"
echo "   后端 Backend:    http://localhost:8080"
echo "   前端 Frontend:   http://localhost:5173"
echo "   爬虫 Scraper API: http://localhost:8000/docs"
echo "   管理 Admin UI:    http://localhost:8000/admin"
echo ""
echo "Press Ctrl+C to stop."

# Wait for processes
wait $BACKEND_PID $FRONTEND_PID $API_PID $WORKER_PID
