# Design: Text Analysis API

Status: **APPROVED** by the user on 2026-10-20 ("zatwierdzam domyślne"): Q-01 = A, Q-02 and Q-03 kept as proposed.
Implements `specs/requirements.md`. Plays the role of the workflow's tech-stack document.

## Summary
| Area | Choice | Version | Why (one line) |
|------|--------|---------|----------------|
| Public API language | Go | 1.24 (`go 1.24` in `go.mod`; toolchain installed here: 1.24.7) | Mandated (C-01); 1.22+ gives `net/http` method/path routing |
| Public API libraries | Standard library only: `net/http`, `encoding/json`, `log/slog`, `context`, `os/signal` | – | C-01, NFR-04; nothing missing for this size |
| Analyzer language | Python | 3.13 | Mandated (C-02); current stable, matches the slim base image |
| Analyzer framework | FastAPI | 0.156.x (pinned exactly in `pyproject.toml`) | Mandated (C-02) |
| ASGI server | Uvicorn | 0.54.x (pinned exactly) | FastAPI's default server; one process is enough on Cloud Run |
| Analyzer test tools (dev only) | pytest, httpx (for FastAPI `TestClient`), ruff | latest at F-01, pinned | Minimal set for tests + lint (C-07) |
| Go tests | `testing` + `net/http/httptest` | – | No test framework needed |
| Go lint | `gofmt -l`, `go vet` | toolchain | Built into the toolchain |
| Containers | Multi-stage Docker builds | – | FR-08; independent images (C-04) |
| Go runtime image | `gcr.io/distroless/static-debian12:nonroot` | pinned tag | Static binary, no shell, non-root (FR-08 AC2) |
| Python runtime image | `python:3.13-slim` + non-root user | pinned tag | Small, official; FastAPI needs a Python runtime |
| Local run | `compose.yaml` (Docker Compose) | – | One command for both services (FR-09) |
| Deploy target | Cloud Run (two services), deploy from source or from image | – | Mandated (C-04); instructions only (C-08) |

## Architecture

```
                      public internet
                            │  HTTPS
                            ▼
              ┌──────────────────────────────┐
              │ Cloud Run service: text-api  │  Go, stdlib
              │  POST /analyze  GET /healthz │
              │  validate → call analyzer    │
              └──────────────┬───────────────┘
                             │  HTTP POST /analyze  (JSON)
                             │  (private: IAM + ID token, Q-01 = A)
                             ▼
              ┌──────────────────────────────┐
              │ Cloud Run service: analyzer  │  Python, FastAPI
              │  POST /analyze  GET /healthz │
              │  pure text analysis          │
              └──────────────────────────────┘
```

Request flow for `POST /analyze` on the API:

1. Method check → `405` (FR-03 AC1).
2. `Content-Type` check (`mime.ParseMediaType`) → `415` (AC2).
3. `http.MaxBytesReader(256 KiB)` → `413` (AC3).
4. `json.Decoder` with `DisallowUnknownFields`, decoding into
   `struct{ Text json.RawMessage }`. Malformed JSON, a non-object, an unknown field, or a second
   `Decode` that does not return `io.EOF` (trailing data) → `400 invalid_json` (AC4, AC5).
   Then `Text` is unmarshalled into a `string`: missing, `null` or a non-string value →
   `400 invalid_text` (AC6).
5. `strings.TrimSpace(text) == ""` → `400 empty_text`; `utf8.RuneCountInString(text) > 10000`
   → `400 text_too_long` (AC7–AC8).
6. Analyzer client call with `context.WithTimeout(r.Context(), ANALYZER_TIMEOUT)` → map
   errors per FR-04.
7. Validate the analyzer's answer (counts ≥ 0, ≤ 5 top words, each count ≥ 1, word non-empty)
   → copy into the API's own response type → `200` (FR-01, D-05).

### Go API layout (`services/api`, module `github.com/krzysztofsikora/gcp-sdd/services/api`)
```
services/api/
  go.mod
  main.go                      config from env, wiring, http.Server, SIGTERM shutdown
  internal/config/config.go    Load(getenv) → Config{Port, AnalyzerURL, AnalyzerTimeout} or error
  internal/analyzer/client.go  Client.Analyze(ctx, text) (Result, error); typed errors
                               ErrTimeout, ErrUnavailable, ErrBadResponse
  internal/analyzer/idtoken.go optional http.RoundTripper adding an ID token from the
                               metadata server, cached until shortly before expiry (Q-01 = A)
  internal/httpapi/handler.go  NewHandler(analyzer Analyzer, logger) http.Handler
                               routes /analyze and /healthz; JSON error helper
  internal/httpapi/validate.go request decoding + validation → (text, *apiError)
  Dockerfile
```
Dependency direction: `main → httpapi → analyzer (via the small Analyzer interface)`; `config`
is used only by `main`. `httpapi` depends on an interface, so its tests use a fake analyzer
and never open a socket; `analyzer` tests use `httptest.Server`.

Error classification in `analyzer.Client`:
- `context.DeadlineExceeded` (or `net.Error.Timeout()`) → `ErrTimeout` → `504 analyzer_timeout`.
- any other transport error (`*url.Error` with dial/DNS failure) → `ErrUnavailable` → `502 analyzer_unavailable`.
- non-200 status, undecodable body, failed result check → `ErrBadResponse` → `502 analyzer_error`.
- No retries (A-06). The `http.Client` is created once and reused (connection pooling).
- A failure to fetch the ID token (only when `ANALYZER_AUTH=google-id-token`) → `ErrUnavailable`
  → `502 analyzer_unavailable`.

Logging (NFR-02): `slog.NewJSONHandler(os.Stdout)`; middleware logs `method`, `path`,
`status`, `duration_ms`. The request body is never logged. Analyzer errors are logged with the
error class, not the text.

Shutdown (NFR-03): `signal.NotifyContext(SIGINT, SIGTERM)` → `server.Shutdown` with a 10 s context.

### Analyzer layout (`services/analyzer`)
```
services/analyzer/
  pyproject.toml               runtime deps: fastapi, uvicorn (exact pins); dev: pytest, httpx, ruff
  app/__init__.py
  app/analysis.py              analyze(text: str) -> AnalysisResult   (pure, no FastAPI imports)
  app/main.py                  FastAPI app: POST /analyze, GET /healthz; pydantic models
  tests/test_analysis.py       FR-02 rules on the pure function
  tests/test_api.py            FR-05, FR-06 via TestClient
  Dockerfile
```
Analysis algorithm (`app/analysis.py`, FR-02):
```python
WORD = re.compile(r"[^\W_]+(?:['’][^\W_]+)*")   # letters/digits, inner ' or ’ allowed
words = WORD.findall(text.lower())
character_count = len(text)                    # code points
word_count = len(words)
top = sorted(Counter(words).items(), key=lambda kv: (-kv[1], kv[0]))[:5]
```
Validation (FR-05 AC2) via pydantic: `text: str` with `min_length=1`, `max_length=10000`
(pydantic counts code points) and a validator rejecting whitespace-only text → FastAPI `422`.
`strict` string type so that numbers are not coerced to strings.

Start command: `uvicorn app.main:app --host 0.0.0.0 --port ${PORT:-8080}` (shell form in the
Dockerfile `CMD` so `$PORT` expands).

## API contracts

### Public: Go API (`text-api`)

`POST /analyze`
```http
POST /analyze HTTP/1.1
Content-Type: application/json

{"text": "The cat and the hat."}
```
`200 OK`, `Content-Type: application/json`
```json
{
  "word_count": 5,
  "character_count": 20,
  "top_words": [
    {"word": "the", "count": 2},
    {"word": "and", "count": 1},
    {"word": "cat", "count": 1},
    {"word": "hat", "count": 1}
  ]
}
```
Errors — always `Content-Type: application/json`, body `{"error": {"code": "...", "message": "..."}}`:

| Status | `code` | When (requirement) |
|--------|--------|--------------------|
| 400 | `invalid_json` | malformed JSON, not an object, trailing data, empty body, unknown field (FR-03 AC4, AC5) |
| 400 | `invalid_text` | `text` missing, `null` or not a string (AC6) |
| 400 | `empty_text` | `text` empty or whitespace only (AC7) |
| 400 | `text_too_long` | more than 10,000 code points (AC8) |
| 404 | `not_found` | any other path |
| 405 | `method_not_allowed` | non-POST on `/analyze`, non-GET on `/healthz`; `Allow` header set (AC1) |
| 413 | `payload_too_large` | body > 262,144 bytes (AC3) |
| 415 | `unsupported_media_type` | `Content-Type` not `application/json` (AC2) |
| 500 | `internal_error` | unexpected server bug (panic recovered) |
| 502 | `analyzer_unavailable` | analyzer unreachable (FR-04 AC2) |
| 502 | `analyzer_error` | analyzer non-200 or invalid body (FR-04 AC3, AC4) |
| 504 | `analyzer_timeout` | analyzer slower than `ANALYZER_TIMEOUT` (FR-04 AC1) |

`GET /healthz` → `200 {"status": "ok"}` (does not call the analyzer).

Configuration (environment):

| Variable | Required | Default | Meaning |
|----------|----------|---------|---------|
| `PORT` | no | `8080` | listen port (set by Cloud Run) |
| `ANALYZER_URL` | **yes** | – | base URL of the analyzer, e.g. `http://localhost:8081`; the client calls `<ANALYZER_URL>/analyze` |
| `ANALYZER_TIMEOUT` | no | `10s` | Go duration; total time for one analyzer call |
| `ANALYZER_AUTH` | no | `none` | `none` or `google-id-token`: attach a Google-signed ID token (audience = `ANALYZER_URL`) from the metadata server (Q-01 = A); any other value → exit non-zero at startup |

### Private: analyzer (`text-analyzer`)

`POST /analyze` — request `{"text": "<1..10000 code points, not whitespace-only>"}`;
`200` response has the same shape as the public `200` above. Invalid input → FastAPI's
standard `422` body (`{"detail": [...]}`); the API treats any non-200 as `analyzer_error`, which
can only happen if the two services disagree on validation.

`GET /healthz` → `200 {"status": "ok"}`.

Configuration: `PORT` (default `8080`; use `8081` locally next to the API).

The contract is owned by this document. Both services' tests use the example bodies above, so a
change to the contract shows up as a failing test on both sides.

## Testing approach (detailed later in `context/test-plan.md`)
- Go: table-driven tests for every FR-03 row (handler with a fake analyzer that records calls,
  asserting it was **not** called); `analyzer` client tests against `httptest.Server` for
  200 / non-200 / bad JSON / slow (timeout) / closed listener (unavailable); `config` tests.
- Python: unit tests of `analyze()` for every FR-02 AC; `TestClient` tests for FR-05 and FR-06.
- Contract: the FR-01 AC1 example is a test fixture on both sides.
- Containers: `docker build` + `/healthz` smoke check (local, not a unit test).
- No test needs GCP or the internet.

## Local development
```bash
# terminal 1 — analyzer
cd services/analyzer && python -m venv .venv && . .venv/bin/activate && pip install -e '.[dev]'
PORT=8081 uvicorn app.main:app --port 8081
# terminal 2 — API
cd services/api && ANALYZER_URL=http://localhost:8081 go run .
# or both at once
docker compose up --build
curl -s localhost:8080/analyze -H 'Content-Type: application/json' -d '{"text":"The cat and the hat."}'
```
`compose.yaml` wires `ANALYZER_URL=http://analyzer:8080` and publishes only the API's port
(the analyzer is private locally too).

Root `Makefile` targets (the workflow gates in `CLAUDE.md`): `lint`, `test`, `docker-build`.

## Cloud Run deployment (outline; full text is `deploy/cloud-run.md`, task S-04)
1. `gcloud run deploy text-analyzer --source services/analyzer --region $REGION` with the
   privacy flags chosen in Q-01.
2. Read its URL (`gcloud run services describe ... --format='value(status.url)'`).
3. `gcloud run deploy text-api --source services/api --region $REGION --allow-unauthenticated
   --set-env-vars ANALYZER_URL=<url>` (+ Q-01 flags).
4. Verify with `curl`; cleanup with `gcloud run services delete`.
Optional notes: `--min-instances=1` on the analyzer against cold starts (R-01, costs money),
`--max-instances` caps.

## Decisions

### Go HTTP routing
- Options considered: stdlib `ServeMux`; chi; gin.
- Chosen: stdlib `ServeMux` (Go 1.22+ patterns), because two routes need no router (C-01, NFR-04).
- Strongest case against: the stdlib's automatic 405/404 bodies are plain text, not our JSON format.
- Mitigation: register `/analyze`, `/healthz` and a catch-all `/` without method patterns and
  check the method in the handler, so every error body is JSON.
- Revisit if: routes grow past a handful or need path parameters plus middleware chains.

### Validation location (D-04)
- Chosen: full validation in Go; same limits re-checked by pydantic in the analyzer.
- Strongest case against: limits live in two places and can drift.
- Why we accept it: two numbers, both pinned by tests on each side and by this document.
- Revisit if: limits change often → serve them from one config value per service.

### Public response mapping (D-05)
- Chosen: decode the analyzer's JSON into a Go type, check it, encode a fresh response.
- Strongest case against: more code than streaming the analyzer's bytes through.
- Why: FR-01 AC3 and FR-04 AC4 need it, and it keeps the public contract ours.

### Analyzer server
- Options: Uvicorn alone; Gunicorn + Uvicorn workers.
- Chosen: Uvicorn, one process; Cloud Run scales by instances, not workers.
- Revisit if: CPU-bound load per instance justifies workers.

### Python packaging
- Options: `requirements.txt`; `pyproject.toml` with pip; uv / poetry.
- Chosen: `pyproject.toml` installed with plain `pip` (no extra tool to learn, C-07).
- Revisit if: lockfile reproducibility becomes a concern → add `uv` lock.

## Decided questions (user, 2026-10-20: "zatwierdzam domyślne")

### Q-01 How should the analyzer be private on Cloud Run? → **A chosen**
| Option | What it means | Code impact | Trade-off |
|--------|---------------|-------------|-----------|
| **A. IAM invoker + ID token (recommended)** | Analyzer deployed with `--no-allow-unauthenticated`; the API's service account gets `roles/run.invoker` on it; the API adds `Authorization: Bearer <ID token>` fetched from the Cloud Run metadata server (audience = `ANALYZER_URL`) | ~40 lines of stdlib Go (metadata server GET + small cache), switched on by `ANALYZER_AUTH=google-id-token`; off locally and in tests | The Cloud Run-native pattern and a good learning target; it is service-to-service platform auth, not end-user auth, so it does not contradict B-20 — but confirm you read B-20 the same way |
| B. Internal ingress + VPC egress | Analyzer `--ingress=internal`; API deployed with Direct VPC egress (`--network`, `--subnet`, `--vpc-egress=all-traffic`) | none | No code, but needs VPC/subnet setup and anything inside the VPC can call the analyzer unauthenticated |
| C. Both A and B | Defence in depth | as A | Most setup; overkill for an MVP |
| D. Public analyzer, "private" by convention | `--allow-unauthenticated` on both | none | Simplest, but does not meet "private" (B-4) |

### Q-02 Limits → **kept**
10,000 characters per text, 256 KiB request body, 10 s analyzer timeout.

### Q-03 Word definition
Letters/digits, inner apostrophes (`'` or `’`), `_` and all other punctuation split words,
digits count as words, case-insensitive, no stop words → **kept**.
