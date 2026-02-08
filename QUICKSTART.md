# PateProject Quick Start

## First Time Setup

Run the setup script once:

```bash
./scripts/setup.sh
```

This will:
- Install/configure PostgreSQL@15
- Setup Node.js 22 via nvm
- Create Python virtual environment
- Install all dependencies
- Create `.env` file

## Starting the Application

```bash
./start.sh
```

Services will be available at:
- **Frontend:** http://localhost:5173
- **Backend:** http://localhost:8080
- **Python Extractor:** http://localhost:8081
- **Database:** localhost:5432

Press `Ctrl+C` to stop all services.

## Configuration

Environment variables are in `.env`:

```bash
# Database
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=pateproject
DB_PORT=5432

# Google OAuth (get from Google Cloud Console)
VITE_GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
```

## Manual Commands

```bash
make db-start    # Start PostgreSQL
make db-stop     # Stop PostgreSQL
make backend     # Run backend only
make frontend    # Run frontend only
make logs        # View backend logs
```

## Troubleshooting

**PostgreSQL not found:**
```bash
brew services start postgresql@15
```

**Node version issues:**
```bash
source ~/.nvm/nvm.sh && nvm use 22
```

**Python dependencies missing:**
```bash
cd python-extractor && source venv/bin/activate && pip install -r requirements.txt
```
