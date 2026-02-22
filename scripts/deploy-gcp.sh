#!/bin/bash

# PateProject GCP Deployment Script
# This script deploys the Backend and Extractor to Google Cloud Run.
# It uses the Dockerfile.gcp artifacts to ensure the original deployment is not affected.

set -e

# Configuration
PROJECT_ID=$(gcloud config get-value project)
REGION="us-central1"
BACKEND_SERVICE="pateproject-backend"
EXTRACTOR_SERVICE="pateproject-extractor"
REPOSITORY="pateproject-repo"

if [ -z "$PROJECT_ID" ]; then
    echo "Error: GCP Project ID not set. Please run 'gcloud config set project [PROJECT_ID]'"
    exit 1
fi

echo "Using Project ID: $PROJECT_ID"
echo "Region: $REGION"

# 1. Create Artifact Registry if it doesn't exist
if ! gcloud artifacts repositories describe $REPOSITORY --location=$REGION >/dev/null 2>&1; then
    echo "Creating Artifact Registry repository: $REPOSITORY..."
    gcloud artifacts repositories create $REPOSITORY \
        --repository-format=docker \
        --location=$REGION \
        --description="Docker repository for PateProject"
fi

# 2. Build and Push Python Extractor
echo "Building and Pushing Python Extractor..."
EXTRACTOR_IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY/$EXTRACTOR_SERVICE"
docker build -t $EXTRACTOR_IMAGE ./python-extractor -f python-extractor/Dockerfile.gcp
docker push $EXTRACTOR_IMAGE

# 3. Build and Push Go Backend
echo "Building and Pushing Go Backend..."
BACKEND_IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY/$BACKEND_SERVICE"
docker build -t $BACKEND_IMAGE ./backend -f backend/Dockerfile.gcp
docker push $BACKEND_IMAGE

# 4. Deploy Extractor to Cloud Run
echo "Deploying Extractor to Cloud Run..."
gcloud run deploy $EXTRACTOR_SERVICE \
    --image $EXTRACTOR_IMAGE \
    --platform managed \
    --region $REGION \
    --allow-unauthenticated \
    --memory 2Gi \
    --cpu 1

# Get Extractor URL
EXTRACTOR_URL=$(gcloud run services describe $EXTRACTOR_SERVICE --platform managed --region $REGION --format='value(status.url)')
echo "Extractor deployed at: $EXTRACTOR_URL"

# 5. Deploy Backend to Cloud Run
echo "Deploying Backend to Cloud Run..."
# Note: You should have set up secrets in GCP Secret Manager for DATABASE_URL, LLM_API_KEY, etc.
# For the first run, we'll deploy with basic env vars. Recommended: use --set-secrets thereafter.
gcloud run deploy $BACKEND_SERVICE \
    --image $BACKEND_IMAGE \
    --platform managed \
    --region $REGION \
    --allow-unauthenticated \
    --set-env-vars "PYTHON_EXTRACTOR_URL=$EXTRACTOR_URL"

BACKEND_URL=$(gcloud run services describe $BACKEND_SERVICE --platform managed --region $REGION --format='value(status.url)')

echo "---------------------------------------------------"
echo "Deployment Complete!"
echo "Backend URL: $BACKEND_URL"
echo "Extractor URL: $EXTRACTOR_URL"
echo ""
echo "NEXT STEPS:"
echo "1. Configure Secrets in GCP Secret Manager (DATABASE_URL, LLM_API_KEY)."
echo "2. Update $BACKEND_SERVICE to use these secrets:"
echo "   gcloud run services update $BACKEND_SERVICE --set-secrets DATABASE_URL=DATABASE_URL:latest,LLM_API_KEY=LLM_API_KEY:latest"
echo "3. Initialize Firebase for the frontend and deploy:"
echo "   cd frontend && firebase init"
echo "---------------------------------------------------"
