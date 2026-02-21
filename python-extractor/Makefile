.PHONY: setup migrate-up migrate-down run-api run-worker run-redis test-scrape search-eggs dev help

# Variables
PYTHON = venv/bin/python
PIP = venv/bin/pip
ALEMBIC = venv/bin/alembic
UVICORN = venv/bin/uvicorn
CELERY = venv/bin/celery

help:
	@echo "Available commands:"
	@echo "  make setup          - Create venv, install dependencies and playwright"
	@echo "  make dev            - Run migrations and start the API server"
	@echo "  make migrate-rev   - Create a new migration revision (usage: make migrate-rev MSG='description')"
	@echo "  make migrate-up    - Upgrade database to latest version"
	@echo "  make migrate-down  - Downgrade database by one version"
	@echo "  make run-api        - Run the FastAPI server"
	@echo "  make run-worker     - Run Celery workers (with 20 concurrency)"
	@echo "  make run-redis      - Start a local Redis container (requires Docker)"

setup:
	@echo "Setting up virtual environment..."
	python3 -m venv venv
	$(PIP) install --upgrade pip
	$(PIP) install -r requirements.txt
	$(PYTHON) -m playwright install chromium

migrate-rev:
	@if [ -z "$(MSG)" ]; then echo "Error: MSG is required. Use 'make migrate-rev MSG=\"your message\"'"; exit 1; fi
	$(ALEMBIC) revision --autogenerate -m "$(MSG)"

migrate-up:
	$(ALEMBIC) upgrade head

migrate-down:
	$(ALEMBIC) downgrade -1

run-api:
	$(UVICORN) app.main:app --host 0.0.0.0 --port 8000 --reload

dev: migrate-up run-api

run-worker:
	$(CELERY) -A app.worker.celery_app worker --loglevel=info -c 20

run-redis:
	docker run -d --name zepto-redis -p 6379:6379 redis:latest

run-postgres:
	docker run -d --name zepto-postgres -e POSTGRES_USER=user -e POSTGRES_PASSWORD=password -e POSTGRES_DB=scraper_db -p 5433:5432 postgres:15

# Trigger initial eggs scrape
search-eggs:
	$(PYTHON) -c "import asyncio; from app.api.endpoints import trigger_search_scrape; asyncio.run(trigger_search_scrape('Eggs', max_products=15))"

seed-categories:
	$(PYTHON) scripts/seed_categories.py

scrape-all:
	$(PYTHON) scripts/scrape_all.py

export-data:
	$(PYTHON) scripts/export_scraped_data.py

monitor:
	$(CELERY) -A app.worker.celery_app flower --port=5555
