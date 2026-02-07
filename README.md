# PateProject

A smart pantry management system that automatically tracks grocery orders from Zepto, Blinkit, and other delivery services. Built with Go, Python, React, and PostgreSQL.

## Features

- 📦 **Automatic Order Ingestion** - Extract items from PDF receipts
- 🐍 **Python PDF Extraction** - Clean extraction using pdfplumber
- 🏪 **Multi-Provider Support** - Zepto, Blinkit, Swiggy Instamart
- 📊 **Pantry Tracking** - Track inventory and low stock items
- 🔐 **OAuth Authentication** - Google OAuth integration
- 🎨 **Modern UI** - React frontend with Vite

## Architecture

```
┌─────────────┐
│   Frontend  │ (React + Vite)
│  Port 5173  │
└──────┬──────┘
       │
┌──────▼──────┐
│  Go Backend │ (Chi Router)
│  Port 8080  │
└──────┬──────┘
       │
       ├──────────────┐
       │              │
┌──────▼──────┐  ┌───▼────────┐
│   Python    │  │ PostgreSQL │
│  Extractor  │  │  Database  │
│  Port 8081  │  │ Port 5432  │
└─────────────┘  └────────────┘
```

## Tech Stack

### Backend
- **Go 1.21+** - Main API server
- **Chi** - HTTP router
- **GORM** - ORM for PostgreSQL
- **Python 3.x** - PDF extraction microservice
- **FastAPI** - Python web framework
- **pdfplumber** - PDF parsing library

### Frontend
- **React 18** - UI framework
- **Vite** - Build tool
- **CSS3** - Styling

### Database
- **PostgreSQL 14+** - Primary database

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Python 3.9 or higher
- PostgreSQL 14 or higher
- Node.js 18 or higher
- npm or yarn

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/pmitra96/pateproject.git
   cd pateproject
   ```

2. **Set up environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Install dependencies**
   ```bash
   # Backend (Go)
   cd backend
   go mod download
   cd ..

   # Python extractor
   cd python-extractor
   pip install -r requirements.txt
   cd ..

   # Frontend
   cd frontend
   npm install
   cd ..
   ```

4. **Start PostgreSQL**
   ```bash
   # Using Homebrew (macOS)
   brew services start postgresql@14

   # Or using Docker
   docker run -d \
     --name pateproject-db \
     -e POSTGRES_PASSWORD=password \
     -e POSTGRES_DB=pateproject \
     -p 5432:5432 \
     postgres:14
   ```

5. **Run the application**
   ```bash
   make dev
   ```

   This starts all services:
   - Backend: http://localhost:8080
   - Python Extractor: http://localhost:8081
   - Frontend: http://localhost:5173
   - Database: localhost:5432

## API Endpoints

### Extraction
- `POST /items/extract` - Extract items from PDF receipt
  - Requires: `Authorization: Bearer <token>`
  - Body: `multipart/form-data` with `invoice` file

### Ingestion
- `POST /ingest/order` - Ingest order data
  - Requires: `X-API-Key` header
  - Body: JSON order data

### Pantry
- `GET /pantry` - Get all pantry items
- `PATCH /pantry/{item_id}` - Update pantry item
- `GET /pantry/low-stock` - Get low stock items

### Items
- `GET /items` - List all items
- `POST /items` - Create new item

## PDF Extraction

The system uses a Python microservice for PDF extraction:

### Supported Providers
- **Zepto** - Full support with unit parsing
- **Blinkit** - Full support
- **Swiggy Instamart** - Basic support

### Unit Parsing
Automatically extracts and normalizes units:
- `(1kg)` → `unit_value: 1000, unit: "g"`
- `500g` → `unit_value: 500, unit: "g"`
- `1 pc` → `unit_value: 1, unit: "pc"`

### Example Response
```json
{
  "provider": "zepto",
  "items": [
    {
      "name": "Akshayakalpa Artisanal Organic Set Curd Cup",
      "count": 1,
      "unit_value": 1000,
      "unit": "g"
    }
  ]
}
```

## Development

### Project Structure
```
pateproject/
├── backend/              # Go backend
│   ├── cmd/             # Application entrypoints
│   ├── controllers/     # HTTP handlers
│   ├── models/          # Database models
│   ├── routes/          # Route definitions
│   ├── extractor/       # PDF extraction (Go fallback)
│   ├── database/        # Database setup
│   ├── middleware/      # Auth middleware
│   └── logger/          # Structured logging
├── python-extractor/    # Python PDF service
│   ├── app.py          # FastAPI application
│   └── requirements.txt # Python dependencies
├── frontend/            # React frontend
│   └── src/
├── scripts/             # Utility scripts
├── dev.sh              # Development startup script
└── Makefile            # Build commands
```

### Makefile Commands

```bash
make dev          # Start all services
make backend      # Start backend only
make frontend     # Start frontend only
make db-start     # Start PostgreSQL
make db-stop      # Stop PostgreSQL
make db-setup     # Create database
make logs         # Tail backend logs
```

### Testing PDF Extraction

```bash
# Test Python service directly
curl -X POST http://localhost:8081/extract \
  -F "file=@zepto.pdf"

# Test via Go backend
curl -X POST http://localhost:8080/items/extract \
  -H "Authorization: Bearer test-token" \
  -F "invoice=@zepto.pdf"
```

## Environment Variables

```bash
# Database
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=pateproject
DB_PORT=5432
DB_SSLMODE=disable

# Services
PYTHON_EXTRACTOR_URL=http://localhost:8081
INGESTION_API_KEY=secret-key

# OAuth (Frontend)
VITE_GOOGLE_CLIENT_ID=your-client-id
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License.

## Acknowledgments

- [pdfplumber](https://github.com/jsvine/pdfplumber) - PDF extraction library
- [Chi](https://github.com/go-chi/chi) - Go HTTP router
- [FastAPI](https://fastapi.tiangolo.com/) - Python web framework
