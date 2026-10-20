# Tasks (roadmap)

Status: **APPROVED** by the user on 2026-10-20 ("zatwierdzam domyślne"): Q-01 = A, Q-02 and Q-03 kept as proposed.
Sources: `specs/requirements.md`, `specs/design.md`.

Execution note (2026-10-20): the user asked for all items in one go without stops. The
`ship` runner cannot publish from this cloud session (`gh` is not logged in), so the items are
implemented directly in the interactive session, one commit per item, on one branch.

Order: the skeleton and gates first, then the analyzer (the API depends on its contract), then
the API, then local wiring and deployment docs. Each item is one change (one branch, one PR) in
the workflow (`docs/workflow.md`); its tasks become the plan's phases. Time estimates add up to
the 1–2 hour budget (B-19); S-03 is the first thing to cut.

## Overview
| ID | Issue | change-id | Type | Covers | Depends on | Size | Status |
|----|-------|-----------|------|--------|------------|------|--------|
| F-01 | #2 | repo-skeleton | foundation | C-01 … C-09, NFR-04 | - | S (~15 min) | done |
| S-01 | #3 | analyzer-service | slice | FR-02, FR-05, FR-06, FR-07 AC1, FR-08 | F-01 | M (~25 min) | done |
| S-02 | #4 | api-analyze | slice | FR-01, FR-03, FR-04, FR-06, FR-07, FR-08, NFR-02, NFR-03 | S-01 | M (~40 min) | done |
| S-03 | #5 | local-compose | slice | FR-09, NFR-01 | S-02 | S (~10 min) | todo |
| S-04 | #6 | cloud-run-deploy | slice | FR-10, Q-01 = A | S-02 | S (~20 min) | todo |

## Items

### F-01 Repository skeleton (`repo-skeleton`)
- Goal: both projects exist, build, and every gate in `CLAUDE.md` runs (and passes on trivial code).
- Covers: C-01 … C-09, NFR-04; design §Go API layout, §Analyzer layout, §Local development.
- Depends on: -
- Tasks:
  - [x] F-01.1 `services/api/go.mod` (module path from design, `go 1.24`), `main.go` that
        starts and exits cleanly; one trivial test.
  - [x] F-01.2 `services/analyzer/pyproject.toml` with exact pins (fastapi, uvicorn; dev:
        pytest, httpx, ruff), `app/__init__.py`, one trivial test.
  - [x] F-01.3 Root `Makefile`: `lint` (`gofmt -l` must print nothing, `go vet ./...`,
        `ruff check`), `test` (`go test ./...`, `pytest`), `docker-build` (placeholder until S-01/S-02).
  - [x] F-01.4 `.github/workflows/ci.yml`: `make lint` and `make test` on every PR (no GCP secrets).
- Acceptance: `make lint` and `make test` exit 0; CI green on the PR.
- Unknowns: -
- Decided up front: versions per design §Summary.
- Size: S

### S-01 Analyzer service (`analyzer-service`)
- Goal: the private Python service computes the analysis and runs in a container.
- Covers: FR-02 AC1–AC6, FR-05 AC1–AC2, FR-06 AC1, FR-07 AC1, FR-08 AC1–AC2 (analyzer image).
- Depends on: F-01
- Tasks:
  - [x] S-01.1 `app/analysis.py`: `analyze(text)` per design algorithm; `tests/test_analysis.py`
        with one test per FR-02 AC (examples copied from the requirements).
  - [x] S-01.2 `app/main.py`: pydantic request/response models, `POST /analyze`,
        `GET /healthz`; `tests/test_api.py` for FR-05 AC1–AC2 and FR-06.
  - [x] S-01.3 `services/analyzer/Dockerfile` (python:3.13-slim, non-root, `$PORT`),
        `.dockerignore`; add to `make docker-build`.
- Acceptance: requirement ACs listed under *Covers*; `docker run -e PORT=8081 -p 8081:8081`
  then `curl localhost:8081/healthz` → `{"status":"ok"}`.
- Unknowns: -
- Decided up front: word rules as proposed (Q-03 kept, user).
- Size: M

### S-02 Go API with validation and analyzer client (`api-analyze`)
- Goal: the public Go service validates input, calls the analyzer, and returns JSON or a mapped error.
- Covers: FR-01 AC1–AC3, FR-03 AC1–AC8, FR-04 AC1–AC5, FR-06 AC1, FR-07 AC1–AC3, FR-08 (API image), NFR-02, NFR-03.
- Depends on: S-01 (contract and example bodies)
- Tasks:
  - [x] S-02.1 `internal/config`: `Load(getenv)` for `PORT`, `ANALYZER_URL`, `ANALYZER_TIMEOUT`
        with tests for defaults and every invalid case (FR-07).
  - [x] S-02.2 `internal/analyzer`: client + typed errors; tests against `httptest.Server`
        for 200, non-200, bad JSON, invalid result, timeout, unavailable (FR-04).
  - [x] S-02.3 `internal/httpapi/validate.go`: decoding and validation; table test with one row
        per FR-03 AC (status, `code`, analyzer not called).
  - [x] S-02.4 `internal/httpapi/handler.go`: `/analyze`, `/healthz`, catch-all `404`, JSON
        error helper, request-log middleware without the body, panic recovery → `500`;
        tests for FR-01 (fake analyzer) and FR-04 mapping.
  - [x] S-02.5 `main.go`: wiring, `http.Server` with timeouts, SIGTERM graceful shutdown (NFR-03).
  - [x] S-02.6 `services/api/Dockerfile` (multi-stage, `CGO_ENABLED=0`, distroless nonroot),
        `.dockerignore`; add to `make docker-build`.
- Acceptance: requirement ACs listed under *Covers*; `go test ./...` with no network beyond
  localhost.
- Unknowns: -
- Decided up front: limits as proposed (Q-02 kept, user); stdlib only (NFR-04).
- Size: M

### S-03 Local end-to-end run (`local-compose`) — Should
- Goal: one command runs both services locally the way they will run on Cloud Run.
- Covers: FR-09 AC1, NFR-01.
- Depends on: S-02
- Tasks:
  - [ ] S-03.1 `compose.yaml`: analyzer (not published) + API (`8080:8080`,
        `ANALYZER_URL=http://analyzer:8080`).
  - [ ] S-03.2 `scripts/smoke.sh`: waits for `/healthz`, posts the FR-01 AC1 example, compares
        the body; README section "Run locally".
- Acceptance: `docker compose up --build -d && scripts/smoke.sh` exits 0.
- Unknowns: Docker availability in CI — smoke runs locally only, not as a gate.
- Decided up front: -
- Size: S

### S-04 Cloud Run deployment instructions (`cloud-run-deploy`)
- Goal: a reviewer can deploy both services to Cloud Run from written steps, analyzer private.
- Covers: FR-10 AC1–AC4; Q-01.
- Depends on: S-02
- Tasks:
  - [ ] S-04.1 `internal/analyzer`: optional ID-token transport
        (`ANALYZER_AUTH=google-id-token`; token from the metadata server, cached until expiry);
        tests with a fake metadata server; off by default.
  - [ ] S-04.2 `deploy/cloud-run.md`: prerequisites (APIs to enable, `gcloud` login — done by
        the person, never by an agent), placeholders `PROJECT_ID`, `REGION`; deploy analyzer →
        read URL → (IAM binding / VPC flags per Q-01) → deploy API → `curl` check → cleanup.
  - [ ] S-04.3 README: link to the deployment doc and a "costs" note (min instances, free tier).
- Acceptance: FR-10 AC1–AC4 reviewed by reading; no command executed by the workflow.
- Unknowns: real Cloud Run behaviour is `UNVERIFIED` until the person deploys (human check in the PR).
- Decided up front: Q-01 = A (user): `--no-allow-unauthenticated` on the analyzer, `roles/run.invoker` for the API's service account, ID token from the metadata server.
- Size: S

## Later
- OpenAPI document for the public API.
- Request ID propagation (`X-Request-ID`) from API to analyzer logs.
- Cloud Build / GitHub Actions deploy pipeline (needs credentials — out of scope now, B-23).
- Mutation testing (decide the tools in `/test-plan`).

## Open roadmap questions
- Q-01, Q-02, Q-03: decided (see *Decided up front*).
- GitHub roadmap: tracking issue #1, one sub-issue per item (#2 … #6).
