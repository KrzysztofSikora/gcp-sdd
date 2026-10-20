# Text Analysis API

Learning project for Go, Python and Google Cloud Run, built with Spec-Driven Development.

```
client ──HTTPS──▶ text-api (Go, public) ──HTTP──▶ text-analyzer (Python FastAPI, private)
```

`POST /analyze {"text": "..."}` → word count, character count and top 5 words as JSON.

| Where | What |
|-------|------|
| `docs/brief.md` | the original brief |
| `specs/requirements.md` | requirements with acceptance criteria |
| `specs/design.md` | architecture, stack, API contracts, open questions |
| `specs/tasks.md` | implementation tasks |
| `deploy/cloud-run.md` | Cloud Run deployment instructions |
| `docs/workflow.md` | the spec-driven, AI-assisted workflow |

## Run locally

With Docker:

```bash
docker compose up --build -d
scripts/smoke.sh          # posts an example and checks the answer
curl -s localhost:8080/analyze -H 'Content-Type: application/json' \
  -d '{"text":"The cat and the hat."}'
docker compose down
```

Without Docker (Go 1.24, Python 3.13):

```bash
make setup                                   # Python venv with dev dependencies
(cd services/analyzer && PORT=8081 .venv/bin/uvicorn app.main:app --port 8081) &
(cd services/api && ANALYZER_URL=http://localhost:8081 go run .)
```

Checks: `make lint`, `make test`, `make docker-build`.

## Deploy to Cloud Run

Step-by-step instructions, with the analyzer kept private through IAM: `deploy/cloud-run.md`.
Nothing is deployed automatically and no GCP credentials are needed to build or test. Both
services scale to zero, so an idle deployment costs close to nothing; the doc's *Costs*
section covers min/max instances.

## API

`POST /analyze` with `Content-Type: application/json` and `{"text": "..."}` (1–10,000
characters) returns `{"word_count", "character_count", "top_words": [{"word", "count"}]}`.
Errors are `{"error": {"code", "message"}}`; codes are listed in `specs/design.md`.
`GET /healthz` returns `{"status": "ok"}`.
