#!/bin/bash

# PateProject GCP Deployment Script
# Deploys:
#   1. Go Backend → Google Cloud Run
#   2. React Frontend → Firebase Hosting (via gcloud)
#
# Usage:
#   bash scripts/deploy-gcp.sh            # Deploy everything
#   bash scripts/deploy-gcp.sh --backend  # Backend only
#   bash scripts/deploy-gcp.sh --frontend # Frontend only

set -e

GCLOUD_PATH="/Users/pushya/Downloads/google-cloud-sdk/bin/gcloud"
PROJECT_ID=$($GCLOUD_PATH config get-value project)
REGION="asia-southeast1"
BACKEND_SERVICE="pateproject-backend"
REPOSITORY="pateproject-repo"

if [ -z "$PROJECT_ID" ]; then
    echo "Error: GCP Project ID not set."
    echo "Run: $GCLOUD_PATH config set project [PROJECT_ID]"
    exit 1
fi

echo "================================================"
echo "  PateProject Deploy → GCP Project: $PROJECT_ID"
echo "================================================"

# Parse flags
DEPLOY_BACKEND=true
DEPLOY_FRONTEND=true
if [ "$1" == "--backend" ]; then
    DEPLOY_FRONTEND=false
elif [ "$1" == "--frontend" ]; then
    DEPLOY_BACKEND=false
fi

# ── Load root .env ──────────────────────────────────────────────────────────
ROOT_ENV="$(dirname "$0")/../.env"
if [ -f "$ROOT_ENV" ]; then
    echo "Loading secrets from $ROOT_ENV..."
    export $(grep -v '^#' "$ROOT_ENV" | grep -v '^$' | xargs)
fi

# ── STEP 1: Ensure Artifact Registry exists ──────────────────────────────────
if ! $GCLOUD_PATH artifacts repositories describe $REPOSITORY --location=$REGION >/dev/null 2>&1; then
    echo "Creating Artifact Registry repository: $REPOSITORY..."
    $GCLOUD_PATH artifacts repositories create $REPOSITORY \
        --repository-format=docker \
        --location=$REGION \
        --description="Docker repository for PateProject"
fi

# ── STEP 2: Docker login to Artifact Registry ────────────────────────────────
echo ""
echo "▶ Authenticating Docker with Artifact Registry..."
# Bypass the gcloud credential helper (which requires gcloud in PATH)
# by writing the auth token directly into Docker config
ACCESS_TOKEN=$($GCLOUD_PATH auth print-access-token)
mkdir -p ~/.docker
# Remove any existing credHelpers entry for this registry and write a direct auth
python3 -c "
import json, base64, os
config_path = os.path.expanduser('~/.docker/config.json')
config = {}
if os.path.exists(config_path):
    with open(config_path) as f:
        config = json.load(f)
# Remove credHelper for this registry if present
config.setdefault('credHelpers', {}).pop('$REGION-docker.pkg.dev', None)
# Write direct auth token
auth = base64.b64encode(f'oauth2accesstoken:$ACCESS_TOKEN'.encode()).decode()
config.setdefault('auths', {})['$REGION-docker.pkg.dev'] = {'auth': auth}
with open(config_path, 'w') as f:
    json.dump(config, f, indent=2)
print('Docker config updated')
"
# Also remove credsStore so Docker uses the auths section directly
python3 -c "
import json, os
config_path = os.path.expanduser('~/.docker/config.json')
with open(config_path) as f:
    config = json.load(f)
config.pop('credsStore', None)
with open(config_path, 'w') as f:
    json.dump(config, f, indent=2)
"
echo "  ✅ Docker authenticated"

# ── STEP 3: Ensure GCP Secrets exist ─────────────────────────────────────────
echo ""
echo "▶ Syncing secrets to GCP Secret Manager..."

create_or_update_secret() {
    local SECRET_NAME=$1
    local SECRET_VALUE=$2

    if [ -z "$SECRET_VALUE" ]; then
        echo "  ⚠️  Skipping $SECRET_NAME (not set in .env)"
        return
    fi

    if $GCLOUD_PATH secrets describe "$SECRET_NAME" >/dev/null 2>&1; then
        echo "  ↻  Updating secret: $SECRET_NAME"
        echo -n "$SECRET_VALUE" | $GCLOUD_PATH secrets versions add "$SECRET_NAME" --data-file=-
    else
        echo "  +  Creating secret: $SECRET_NAME"
        echo -n "$SECRET_VALUE" | $GCLOUD_PATH secrets create "$SECRET_NAME" --data-file=-
    fi
}

create_or_update_secret "DATABASE_URL" "$DATABASE_URL"
create_or_update_secret "LLM_API_KEY" "$LLM_API_KEY"
create_or_update_secret "INGESTION_API_KEY" "$INGESTION_API_KEY"
create_or_update_secret "ALLOWED_ORIGINS" "$ALLOWED_ORIGINS"
create_or_update_secret "WHATSAPP_VERIFY_TOKEN" "$WHATSAPP_VERIFY_TOKEN"
create_or_update_secret "WHATSAPP_ACCESS_TOKEN" "$WHATSAPP_ACCESS_TOKEN"
create_or_update_secret "WHATSAPP_PHONE_NUMBER_ID" "$WHATSAPP_PHONE_NUMBER_ID"
create_or_update_secret "JWT_SECRET" "$JWT_SECRET"

# ── STEP 4: Deploy Backend ────────────────────────────────────────────────────
if [ "$DEPLOY_BACKEND" = true ]; then
    echo ""
    echo "▶ Building and pushing Go Backend (linux/amd64)..."
    BACKEND_IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY/$BACKEND_SERVICE"
    docker build --platform linux/amd64 -t $BACKEND_IMAGE ./backend -f backend/Dockerfile.gcp
    docker push $BACKEND_IMAGE

    echo "▶ Deploying Backend to Cloud Run..."
    $GCLOUD_PATH run deploy $BACKEND_SERVICE \
        --image $BACKEND_IMAGE \
        --platform managed \
        --region $REGION \
        --allow-unauthenticated \
        --memory 512Mi \
        --cpu 1 \
        --set-secrets="DATABASE_URL=DATABASE_URL:latest,LLM_API_KEY=LLM_API_KEY:latest,INGESTION_API_KEY=INGESTION_API_KEY:latest,ALLOWED_ORIGINS=ALLOWED_ORIGINS:latest,WHATSAPP_VERIFY_TOKEN=WHATSAPP_VERIFY_TOKEN:latest,WHATSAPP_ACCESS_TOKEN=WHATSAPP_ACCESS_TOKEN:latest,WHATSAPP_PHONE_NUMBER_ID=WHATSAPP_PHONE_NUMBER_ID:latest,JWT_SECRET=JWT_SECRET:latest"

    BACKEND_URL=$($GCLOUD_PATH run services describe $BACKEND_SERVICE --platform managed --region $REGION --format='value(status.url)')
    echo "  ✅ Backend deployed: $BACKEND_URL"
fi

# ── STEP 4: Deploy Frontend to Firebase Hosting ───────────────────────────────
if [ "$DEPLOY_FRONTEND" = true ]; then
    echo ""
    echo "▶ Setting up Firebase Hosting..."

    # Install Firebase CLI if not present
    if ! command -v firebase &>/dev/null; then
        echo "  Installing Firebase CLI..."
        npm install -g firebase-tools
    fi

    # Determine backend URL
    if [ -z "$BACKEND_URL" ]; then
        BACKEND_URL=$($GCLOUD_PATH run services describe $BACKEND_SERVICE --platform managed --region $REGION --format='value(status.url)' 2>/dev/null || echo "")
    fi

    if [ -z "$BACKEND_URL" ]; then
        echo "  ⚠️  Could not determine backend URL. Set VITE_API_BASE_URL manually in frontend/.env.production"
    else
        echo "  Backend URL: $BACKEND_URL"
    fi

    # Write frontend production .env
    FRONTEND_ENV="./frontend/.env.production"
    echo "  Writing $FRONTEND_ENV..."
    cat > "$FRONTEND_ENV" << EOF
VITE_API_BASE_URL=$BACKEND_URL
VITE_GOOGLE_CLIENT_ID=${VITE_GOOGLE_CLIENT_ID:-$VITE_GOOGLE_CLIENT_ID}
EOF

    # Build the frontend
    echo "  Building React app..."
    cd frontend
    npm install --silent
    npm run build
    cd ..

    # Initialize Firebase if no .firebaserc exists
    if [ ! -f ".firebaserc" ]; then
        echo ""
        echo "  ⚙️  First-time Firebase setup required."
        echo "  Running: firebase init hosting"
        echo "  - Select project: $PROJECT_ID"
        echo "  - Public directory: frontend/dist"
        echo "  - Single-page app: Yes"
        echo "  - Don't overwrite dist/index.html"
        echo ""
        firebase login
        firebase init hosting --project $PROJECT_ID
    fi

    echo "  Deploying to Firebase Hosting..."
    firebase deploy --only hosting --project $PROJECT_ID

    HOSTING_URL="https://$PROJECT_ID.web.app"
    echo "  ✅ Frontend deployed: $HOSTING_URL"

    # Update backend ALLOWED_ORIGINS with the Firebase URL
    echo ""
    echo "  Updating ALLOWED_ORIGINS to include Firebase URL..."
    UPDATED_ORIGINS="${ALLOWED_ORIGINS:+$ALLOWED_ORIGINS,}$HOSTING_URL"
    echo -n "$UPDATED_ORIGINS" | $GCLOUD_PATH secrets versions add "ALLOWED_ORIGINS" --data-file=-
    $GCLOUD_PATH run services update $BACKEND_SERVICE \
        --region $REGION \
        --update-secrets="ALLOWED_ORIGINS=ALLOWED_ORIGINS:latest"
    echo "  ✅ CORS updated."
fi

# ── SUMMARY ───────────────────────────────────────────────────────────────────
echo ""
echo "================================================"
echo "  🚀 Deployment Complete!"
if [ "$DEPLOY_BACKEND" = true ] && [ -n "$BACKEND_URL" ]; then
    echo "  Backend:  $BACKEND_URL"
fi
if [ "$DEPLOY_FRONTEND" = true ]; then
    echo "  Frontend: https://$PROJECT_ID.web.app"
fi
echo "================================================"
echo ""
echo "NEXT STEPS:"
echo "  1. Add your Firebase Hosting URL to Google OAuth allowed origins"
echo "     → https://console.cloud.google.com/apis/credentials"
echo "  2. Test: curl $BACKEND_URL/health"
echo "================================================"
