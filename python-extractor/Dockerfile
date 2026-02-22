FROM mcr.microsoft.com/playwright/python:v1.40.0-jammy

WORKDIR /app

# Install system dependencies
# ... (previous lines)
RUN apt-get update && apt-get install -y \
    gcc \
    libpq-dev \
    tesseract-ocr \
    libgl1 \
    make \
    && rm -rf /var/lib/apt/lists/*
# ... (rest of the file)
CMD ["make", "prod-start"]
