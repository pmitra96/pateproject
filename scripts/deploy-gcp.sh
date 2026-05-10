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
#   bash scripts/deploy-gcp.sh --backend --build-mode cloud  # Backend with Cloud Build

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
COST_MODE="NORMAL"
BUILD_MODE="local"
SECRET_MODE_ARG=""
for arg in "$@"; do
    if [ "$arg" == "--backend" ]; then
        DEPLOY_FRONTEND=false
    elif [ "$arg" == "--frontend" ]; then
        DEPLOY_BACKEND=false
    fi
done
prev=""
for i in "$@"; do
    if [ "$prev" = "--cost-mode" ]; then
        COST_MODE="$i"
        break
    fi
    prev="$i"
done
prev=""
for i in "$@"; do
    if [ "$prev" = "--build-mode" ]; then
        BUILD_MODE="$i"
        break
    fi
    prev="$i"
done
prev=""
for i in "$@"; do
    if [ "$prev" = "--secret-mode" ]; then
        SECRET_MODE_ARG="$i"
        break
    fi
    prev="$i"
done

if [ "$BUILD_MODE" != "local" ] && [ "$BUILD_MODE" != "cloud" ]; then
    echo "Error: --build-mode must be either 'local' or 'cloud' (got '$BUILD_MODE')"
    exit 1
fi
if [ -n "$SECRET_MODE_ARG" ] && [ "$SECRET_MODE_ARG" != "env" ]; then
    echo "Error: only env-only deployment is supported."
    exit 1
fi
if [ "$SECRET_MODE_ARG" = "env" ]; then
    echo "Note: --secret-mode env is now default and the only supported mode."
fi

CPU="1"
MEMORY="512Mi"
MIN_INSTANCES="0"
MAX_INSTANCES="10"
CONCURRENCY="80"
TIMEOUT="30"
if [ "$COST_MODE" = "LOW" ]; then
    MEMORY="256Mi"
    MAX_INSTANCES="1"
fi

# ── Load root .env ──────────────────────────────────────────────────────────
ROOT_ENV="$(dirname "$0")/../.env"
if [ -f "$ROOT_ENV" ]; then
    echo "Loading secrets from $ROOT_ENV..."
    export $(grep -v '^#' "$ROOT_ENV" | grep -v '^$' | xargs)
fi

# Force OpenAI-only provider for all deployments.
LLM_PROVIDER="openai"
if [ -z "$OPENAI_MODEL" ]; then
    OPENAI_MODEL="gpt-4o-mini"
fi

build_env_yaml() {
    local env_file=$1
    python3 - "$env_file" <<'PY'
import os, sys
path = sys.argv[1]
keys = [
  "DATABASE_URL",
  "PYTHON_EXTRACTOR_URL",
  "OPENAI_API_KEY",
  "LLM_PROVIDER",
  "OPENAI_MODEL",
  "INGESTION_API_KEY",
  "ALLOWED_ORIGINS",
  "WHATSAPP_VERIFY_TOKEN",
  "WHATSAPP_ACCESS_TOKEN",
  "WHATSAPP_PHONE_NUMBER_ID",
  "JWT_SECRET",
]
with open(path, "w", encoding="utf-8") as f:
    for k in keys:
        v = os.environ.get(k, "")
        if v is None:
            v = ""
        v = v.replace("'", "''")
        f.write(f"{k}: '{v}'\n")
PY
}

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

# ── STEP 3: Secret handling ──────────────────────────────────────────────────
echo ""
echo "▶ Secret mode: env-only"

# ── STEP 4: Deploy Backend ────────────────────────────────────────────────────
if [ "$DEPLOY_BACKEND" = true ]; then
    echo ""
    echo "▶ Building and pushing Go Backend (linux/amd64) [mode=$BUILD_MODE]..."
    BACKEND_IMAGE="$REGION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY/$BACKEND_SERVICE"
    if [ "$BUILD_MODE" = "cloud" ]; then
        $GCLOUD_PATH builds submit ./backend --tag $BACKEND_IMAGE
    else
        docker build --platform linux/amd64 -t $BACKEND_IMAGE ./backend -f backend/Dockerfile.gcp
        docker push $BACKEND_IMAGE
    fi

    echo "▶ Deploying Backend to Cloud Run..."
    echo "  Clearing existing secret-based env bindings (if any)..."
    $GCLOUD_PATH run services update $BACKEND_SERVICE \
        --region $REGION \
        --clear-secrets >/dev/null 2>&1 || true
    ENV_YAML=$(mktemp)
    build_env_yaml "$ENV_YAML"
    $GCLOUD_PATH run deploy $BACKEND_SERVICE \
        --image $BACKEND_IMAGE \
        --platform managed \
        --region $REGION \
        --allow-unauthenticated \
        --memory $MEMORY \
        --cpu $CPU \
        --min-instances $MIN_INSTANCES \
        --max-instances $MAX_INSTANCES \
        --concurrency $CONCURRENCY \
        --timeout $TIMEOUT \
        --env-vars-file "$ENV_YAML"
    rm -f "$ENV_YAML"

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
    $GCLOUD_PATH run services update $BACKEND_SERVICE \
        --region $REGION \
        --update-env-vars="^:^ALLOWED_ORIGINS=$UPDATED_ORIGINS"
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
