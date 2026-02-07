.PHONY: dev db-start db-stop db-setup backend frontend install help

# Default command: run everything locally
dev:
	./dev.sh

# Infrastructure (macOS/Homebrew)
db-start:
	brew services start postgresql@15 || brew services start postgresql

db-stop:
	brew services stop postgresql@15 || brew services stop postgresql

db-setup:
	createdb pateproject || echo "Database already exists"

# Utility
ingest-sample:
	python3 scripts/ingest_order.py scripts/samples/zepto_order.json

logs:
	tail -f backend.log

logs-frontend:
	tail -f frontend.log

# Individual services
backend:
	cd backend && go run ./cmd/server/main.go

frontend:
	cd frontend && npm run dev

# Installation
install:
	cd frontend && npm install
	cd backend && go mod download

# Help
help:
	@echo "Available commands:"
	@echo "  make dev       - Run dev cluster (auto-creates DB if missing)"
	@echo "  make db-setup  - Create the pateproject database manually"
	@echo "  make db-start  - Start local PostgreSQL (Homebrew)"
	@echo "  make db-stop   - Stop local PostgreSQL (Homebrew)"
	@echo "  make backend   - Run only the backend"
	@echo "  make frontend  - Run only the frontend"
	@echo "  make install   - Install dependencies for all services"
