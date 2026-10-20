# Deploying to Cloud Run

Two independent Cloud Run services (FR-10):

| Service | Source | Who can call it |
|---------|--------|-----------------|
| `text-analyzer` | `services/analyzer` | only the `text-api` service account (IAM, `--no-allow-unauthenticated`) |
| `text-api` | `services/api` | anyone (`--allow-unauthenticated`) |

The API proves its identity to the analyzer with a Google-signed ID token from the Cloud Run
metadata server (`ANALYZER_AUTH=google-id-token`, decision Q-01 = A in `specs/design.md`). No
keys or secrets are created or stored.

> Run these commands yourself, from the repository root. Nothing in this repository runs them,
> and building or testing the code never needs GCP credentials.

## Prerequisites

- A Google Cloud project with billing enabled, and the `gcloud` CLI installed.
- Log in and pick the project:

```bash
gcloud auth login
gcloud config set project PROJECT_ID
```

Set the values used below (replace the placeholders):

```bash
export PROJECT_ID=PROJECT_ID          # e.g. my-text-api-123
export REGION=REGION                  # e.g. europe-central2
export API_SA=text-api-sa@${PROJECT_ID}.iam.gserviceaccount.com
export ANALYZER_SA=text-analyzer-sa@${PROJECT_ID}.iam.gserviceaccount.com
```

Enable the APIs (once per project):

```bash
gcloud services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com
```

Create one service account per service, with no roles of its own (least privilege):

```bash
gcloud iam service-accounts create text-api-sa --display-name "text-api runtime"
gcloud iam service-accounts create text-analyzer-sa --display-name "text-analyzer runtime"
```

## 1. Deploy the analyzer (private)

```bash
gcloud run deploy text-analyzer \
  --source services/analyzer \
  --region "$REGION" \
  --service-account "$ANALYZER_SA" \
  --no-allow-unauthenticated
```

`--source` builds the service's `Dockerfile` with Cloud Build and stores the image in Artifact
Registry; on the first run `gcloud` offers to create the `cloud-run-source-deploy` repository —
answer yes.

Read its URL:

```bash
export ANALYZER_URL=$(gcloud run services describe text-analyzer --region "$REGION" --format='value(status.url)')
echo "$ANALYZER_URL"
```

## 2. Let the API call the analyzer

```bash
gcloud run services add-iam-policy-binding text-analyzer \
  --region "$REGION" \
  --member "serviceAccount:${API_SA}" \
  --role roles/run.invoker
```

## 3. Deploy the API (public)

```bash
gcloud run deploy text-api \
  --source services/api \
  --region "$REGION" \
  --service-account "$API_SA" \
  --allow-unauthenticated \
  --set-env-vars "ANALYZER_URL=${ANALYZER_URL},ANALYZER_AUTH=google-id-token"
```

Optional: `--set-env-vars ...,ANALYZER_TIMEOUT=15s` if analyzer cold starts are slow (see Costs).

If `--allow-unauthenticated` fails with an organisation policy error (domain restricted
sharing), your organisation does not allow public services; ask its admin, or test the API with
`curl -H "Authorization: Bearer $(gcloud auth print-identity-token)"` instead.

## 4. Verify

```bash
export API_URL=$(gcloud run services describe text-api --region "$REGION" --format='value(status.url)')

curl -s "$API_URL/healthz"
# {"status":"ok"}

curl -s "$API_URL/analyze" -H 'Content-Type: application/json' -d '{"text":"The cat and the hat."}'
# {"word_count":5,"character_count":20,"top_words":[{"word":"the","count":2},{"word":"and","count":1},{"word":"cat","count":1},{"word":"hat","count":1}]}

API_URL=$API_URL scripts/smoke.sh
# smoke: OK ...

# The analyzer is private: a direct call without a token is refused.
curl -s -o /dev/null -w '%{http_code}\n' "$ANALYZER_URL/healthz"
# 403
```

If `/analyze` returns `502 analyzer_unavailable`, check the API's logs; a missing
`roles/run.invoker` binding shows up as `analyzer_error` (the analyzer answers 403):

```bash
gcloud logging read \
  'resource.type="cloud_run_revision" AND resource.labels.service_name="text-api"' \
  --limit 20 --format='value(jsonPayload)'
```

## Redeploying one service

Each service is deployed on its own (C-04): re-run step 1 or step 3 only. The analyzer's URL
does not change between revisions, so the API does not need redeploying when the analyzer changes.

## Costs

- Both services scale to zero by default; with no traffic you pay only for stored images
  (Artifact Registry) — within the free tier for a project this size.
- The first request after idle waits for a cold start of both services (typically a few
  seconds). `--min-instances 1` on `text-analyzer` removes the analyzer's cold start but is
  billed continuously; leave it at 0 for learning.
- Add `--max-instances 2` to both deploys to cap spending on a public endpoint.

## Clean up

```bash
gcloud run services delete text-api --region "$REGION" --quiet
gcloud run services delete text-analyzer --region "$REGION" --quiet
gcloud iam service-accounts delete "$API_SA" --quiet
gcloud iam service-accounts delete "$ANALYZER_SA" --quiet
gcloud artifacts repositories delete cloud-run-source-deploy --location "$REGION" --quiet
```

The last command removes all images that `--source` deploys created in that region; skip it if
other services use the same repository.
