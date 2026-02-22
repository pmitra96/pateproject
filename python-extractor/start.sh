#!/bin/bash

# Use the port provided by environment or default to 7860 for HF
export PORT=${PORT:-7860}

# Start Celery worker in the background using Makefile
echo "Starting Celery worker via Makefile..."
make run-worker &

# Start FastAPI server using Makefile
echo "Starting FastAPI server via Makefile..."
make run-api
