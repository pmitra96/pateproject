#!/bin/bash
# First-time setup script for PateProject
# Run this once: ./scripts/setup.sh

set -e

echo "🔧 PateProject First-Time Setup"
echo "================================"

# 1. Check and install PostgreSQL
echo ""
echo "📦 Checking PostgreSQL..."
if ! brew list postgresql@15 &>/dev/null; then
    echo "   Installing PostgreSQL@15..."
    brew install postgresql@15
else
    echo "   ✅ PostgreSQL@15 already installed"
fi

# Start PostgreSQL if not running
export PATH="/opt/homebrew/opt/postgresql@15/bin:$PATH"
if ! pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
    echo "   Starting PostgreSQL..."
    brew services start postgresql@15
    sleep 2
fi

# Create postgres user if it doesn't exist
if ! psql -U postgres -c '' 2>/dev/null; then
    echo "   Creating postgres user..."
    createuser -s postgres 2>/dev/null || true
fi
echo "   ✅ PostgreSQL is ready"

# 2. Setup Node.js via nvm
echo ""
echo "📦 Checking Node.js..."
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

if ! command -v nvm &>/dev/null; then
    echo "   ❌ nvm not found. Please install nvm first:"
    echo "   curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash"
    exit 1
fi

# Install and use Node 22
nvm install 22 2>/dev/null || true
nvm use 22
echo "   ✅ Node.js $(node --version) ready"

# 3. Setup Python virtual environment
echo ""
echo "🐍 Setting up Python environment..."
if [ ! -d "python-extractor/venv" ]; then
    echo "   Creating virtual environment..."
    python3 -m venv python-extractor/venv
fi
source python-extractor/venv/bin/activate
pip install -q -r python-extractor/requirements.txt
deactivate
echo "   ✅ Python environment ready"

# 4. Install frontend dependencies
echo ""
echo "🎨 Installing frontend dependencies..."
(cd frontend && npm install --silent)
echo "   ✅ Frontend dependencies installed"

# 5. Install backend dependencies
echo ""
echo "📦 Installing backend dependencies..."
(cd backend && go mod download)
echo "   ✅ Backend dependencies installed"

# 6. Setup .env file
echo ""
echo "⚙️  Checking environment configuration..."
if [ ! -f .env ]; then
    cp .env.example .env
    echo "   Created .env from .env.example"
    echo "   ⚠️  Remember to update VITE_GOOGLE_CLIENT_ID in .env"
else
    echo "   ✅ .env already exists"
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "To start the application, run:"
echo "   ./start.sh"
echo ""
