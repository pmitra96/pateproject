---
title: PateProject Extractor
emoji: 📦
colorFrom: blue
colorTo: green
sdk: docker
pinned: false
---

# PateProject Extractor

This is the Python microservice for the PateProject. It handles:
- PDF Extraction (pdfplumber)
- Web Scraping (Playwright)
- Nutrition Processing (Celery Worker)

## Configuration

This Space requires the following environment variables (Secrets):
- `DATABASE_URL`: Connection string to your Neon Postgres DB.
- `REDIS_URL`: Connection string to your Upstash Redis.

## Deployment

Simply deploy as a Docker Space. The `Dockerfile` handles the installation of Playwright and all dependencies.
