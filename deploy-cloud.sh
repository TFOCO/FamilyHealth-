#!/usr/bin/env bash
set -e

PROJECT_ID="aevora-familyhealth-prod"
REGION="us-central1"
IMAGE_NAME="gcr.io/${PROJECT_ID}/fastenhealth-api:latest"

echo "====================================================="
echo " Deploying FamilyHealth+ Backend to Google Cloud Run"
echo "====================================================="

# 1. Ensure gcloud is configured
gcloud config set project ${PROJECT_ID} || echo "Project may not exist yet, creating..."
gcloud projects create ${PROJECT_ID} --name="FamilyHealth Plus" --set-as-default || true

# 2. Enable APIs
gcloud services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com

# 3. Build the Backend using Cloud Build
gcloud builds submit --tag ${IMAGE_NAME} .

# 4. Deploy Backend to Cloud Run
# Note: In production, configure Cloud SQL connection or Cloud Storage FUSE for the SQLite DB
gcloud run deploy fastenhealth-api \
  --image ${IMAGE_NAME} \
  --region ${REGION} \
  --platform managed \
  --allow-unauthenticated \
  --port 8080

echo "====================================================="
echo " Deploying Frontend to Firebase Hosting"
echo "====================================================="

# 5. Export React Native Web
cd mobile
yarn install
npx expo export:web
cd ..

# 6. Deploy to Firebase
firebase deploy --only hosting --project ${PROJECT_ID}

echo "Deployment Complete!"
