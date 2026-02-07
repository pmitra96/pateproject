#!/bin/bash

# Function to kill background processes on script exit
cleanup() {
    echo ""
    echo "🛑 Stopping all services..."
    
    # Send SIGTERM to the captured PIDs
    if [ ! -z "$BACKEND_PID" ]; then kill $BACKEND_PID 2>/dev/null; fi
    if [ ! -z "$FRONTEND_PID" ]; then kill $FRONTEND_PID 2>/dev/null; fi
    
    # Fallback: Kill processes on specific ports
    lsof -i :8080 -t | xargs kill -9 2>/dev/null
    lsof -i :5173 -t | xargs kill -9 2>/dev/null
    
    echo "✅ Cleanup complete."
}
trap cleanup EXIT INT TERM

# Load environment variables
if [ -f .env ]; then
  set -a
  source .env
  set +a
fi

echo "🚀 Starting PateProject Development Environment (Local Only)..."

# 1. Check Database
echo "🐘 Checking Local Database (PostgreSQL)..."
# We assume DB is already running
if ! pg_isready -h localhost -p 5432 -U postgres > /dev/null 2>&1; then
  echo "❌ PostgreSQL is not running on localhost:5432."
  echo "   Try 'make db-start' if you use Homebrew, or start your local Postgres app."
  exit 1
fi

# Check if pateproject database exists, create if not
if ! psql -h localhost -U postgres -lqt | cut -d \| -f 1 | grep -qw pateproject; then
  echo "🛠️ Database 'pateproject' not found. Creating it..."
  createdb -h localhost -U postgres pateproject || { echo "❌ Failed to create database"; exit 1; }
fi

echo "✅ Database is ready!"

# 2. Start Backend
echo "📦 Starting Backend (Go)... [Logs: backend.log]"
(cd backend && go run ./cmd/server/main.go) > backend.log 2>&1 &
BACKEND_PID=$!

# 3. Start Python Extractor
echo "🐍 Starting PDF Extractor (Python)... [Logs: python-extractor.log]"
(cd python-extractor && python3 app.py) > python-extractor.log 2>&1 &
PYTHON_PID=$!

# 4. Start Frontend
echo "🎨 Starting Frontend (React)... [Logs: frontend.log]"
(cd frontend && npm run dev) > frontend.log 2>&1 &
FRONTEND_PID=$!

echo ""
echo "✅ All services started!"
echo "   Backend:  http://localhost:8080"
echo "   Python:   http://localhost:8081"
echo "   Frontend: http://localhost:5173"
echo "   Database: localhost:5432"
echo ""
echo "Press Ctrl+C to stop."

# Wait for processes
wait $BACKEND_PID $PYTHON_PID $FRONTEND_PID
