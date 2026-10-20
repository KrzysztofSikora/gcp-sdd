# Requirements (PRD): Text Analysis API

Status: **DRAFT, awaiting user approval** (B-15).
Sources: `docs/brief.md` (B-n), `context/shape-notes.md` (D-nn, A-nn, NG-nn, Q-nn).
This file says **what** is built and how we know it works. How it is built is in
`specs/design.md`.

## Problem and goal
Learn Go, Python and Google Cloud Run while practising spec-driven, AI-assisted engineering
(B-1), by building a small Text Analysis API (B-2). A client sends a text and gets back its word
count, character count and five most frequent words.

## Users
- **API client**: any HTTP client (curl, a test, a script). Sends text, reads JSON.
- **Developer/operator** (the user): runs, tests and later deploys the services and reads their logs.

## Constraints
- C-01 Public REST API written in **Go**; prefer the Go standard library (B-3, B-21).
- C-02 Private analysis service written in **Python** with **FastAPI** (B-4).
- C-03 Services talk over **HTTP** (B-5).
- C-04 Two **independently deployable Cloud Run services**, each with its own container image (B-6, B-17).
- C-05 MVP achievable in **1–2 hours** (B-19).
- C-06 No frontend, database, authentication or message queue (B-20).
- C-07 Minimal dependencies (B-22).
- C-08 No deployment and no GCP credentials needed to build, test or run locally (B-23).
- C-09 No secrets in the repository (B-24).

## User stories
- US-01 As an API client, I send a text to `POST /analyze` and receive its word count,
  character count and top 5 words as JSON. (B-7, B-10, B-11)
- US-02 As an API client, when my request is invalid I get a clear 4xx JSON error that says
  what is wrong, and the analyzer is not called. (B-8)
- US-03 As an API client, when the analysis service fails or is too slow I get a 5xx JSON error
  instead of a hang or a crash. (B-9)
- US-04 As the operator, I can check that each service is up with a health endpoint. (B-6)
- US-05 As the operator, I can build, test and run both services locally with no GCP account. (B-17, B-23)
- US-06 As the operator, I can follow written instructions to deploy each service to Cloud Run
  on its own, with the analysis service not publicly callable. (B-4, B-6, B-18)

## Functional requirements

### FR-01 Analyze text (Must, covers US-01, B-7, B-9, B-10, B-11)
- AC1: Given the API and analyzer are running, When a client sends `POST /analyze` with
  `Content-Type: application/json` and body `{"text": "The cat and the hat."}`, Then the
  response is `200`, `Content-Type: application/json`, and the body is
  `{"word_count": 5, "character_count": 20, "top_words": [{"word": "the", "count": 2},
  {"word": "and", "count": 1}, {"word": "cat", "count": 1}, {"word": "hat", "count": 1}]}`.
- AC2: Given a valid request, When the API handles it, Then it sends exactly one HTTP request
  to the analyzer carrying the same text, and the response fields come from the analyzer's answer.
- AC3: Given a valid request, When the response is returned, Then its JSON object has exactly
  the keys `word_count`, `character_count` and `top_words`, even if the analyzer adds others.

### FR-02 Text analysis rules (Must, covers US-01, B-10)
Implemented by the analyzer; observable through FR-01.
- AC1 (characters): Given text `"héllo wörld"`, Then `character_count` is `11` (Unicode code
  points, including spaces and punctuation; A-01).
- AC2 (words): Given text `"Don't stop—don't! 2024 is_here"`, Then the words are
  `don't, stop, don't, 2024, is, here` and `word_count` is `6` (A-03).
- AC3 (case): Given text `"Go go GO"`, Then `top_words` is `[{"word": "go", "count": 3}]` (A-02).
- AC4 (top 5, order): Given text `"b a c b a d e f b"`, Then `top_words` is
  `[{"word":"b","count":3},{"word":"a","count":2},{"word":"c","count":1},{"word":"d","count":1},{"word":"e","count":1}]`
  — sorted by count descending, ties alphabetically, at most 5 entries (A-04).
- AC5 (fewer than 5): Given text with only 2 distinct words, Then `top_words` has 2 entries.
- AC6 (no words): Given text `"!!! ???"`, Then `word_count` is `0`, `character_count` is `7`,
  and `top_words` is `[]`.

### FR-03 Request validation in the Go API (Must, covers US-02, B-8)
For every case below the response is JSON `{"error": {"code": "<code>", "message": "<text>"}}`
and **the analyzer receives no request**.
- AC1: Given a method other than `POST` on `/analyze`, Then `405`, header `Allow: POST`, code `method_not_allowed`.
- AC2: Given a `Content-Type` other than `application/json` (parameters such as
  `; charset=utf-8` are allowed), Then `415`, code `unsupported_media_type`.
- AC3: Given a body larger than 256 KiB, Then `413`, code `payload_too_large`.
- AC4: Given a body that is not a single JSON object (malformed JSON, an array, trailing data,
  an empty body), Then `400`, code `invalid_json`.
- AC5: Given a JSON object with a field other than `text`, Then `400`, code `invalid_json`.
- AC6: Given `text` missing, `null`, or not a string, Then `400`, code `invalid_text`.
- AC7: Given `text` that is empty or only whitespace, Then `400`, code `empty_text`.
- AC8: Given `text` longer than 10,000 characters (code points), Then `400`, code
  `text_too_long`; exactly 10,000 characters is accepted (A-05).

### FR-04 Analyzer failure handling (Must, covers US-03, B-9)
- AC1: Given the analyzer does not answer within the configured timeout (default 10 s), When a
  valid request arrives, Then the API responds `504` with code `analyzer_timeout` and does not retry.
- AC2: Given the analyzer is unreachable (connection refused, DNS failure), Then `502`, code `analyzer_unavailable`.
- AC3: Given the analyzer answers with a non-`200` status, Then `502`, code `analyzer_error`.
- AC4: Given the analyzer answers `200` with a body that is not valid JSON, misses a field, has a
  negative count, or more than 5 `top_words`, Then `502`, code `analyzer_error`.
- AC5: In every case above the API process keeps serving later requests.

### FR-05 Analyzer service contract (Must, covers US-01, B-4, B-10)
- AC1: Given the analyzer is running, When `POST /analyze` is sent to it with `{"text": "a b a"}`,
  Then it answers `200` with `{"word_count": 3, "character_count": 5, "top_words":
  [{"word": "a", "count": 2}, {"word": "b", "count": 1}]}`.
- AC2 (defensive validation, D-04): Given `text` missing, not a string, empty/whitespace-only or
  longer than 10,000 characters, Then the analyzer answers `422` and computes nothing.

### FR-06 Health endpoints (Must, covers US-04)
- AC1: Given either service is running, When `GET /healthz` is sent, Then `200` with body
  `{"status": "ok"}`. The API's health check does **not** call the analyzer.

### FR-07 Runtime configuration (Must, covers US-05, US-06, C-04)
- AC1: Given environment variable `PORT`, When a service starts, Then it listens on that port;
  default `8080` when unset.
- AC2: Given the API starts without `ANALYZER_URL`, or with a value that is not an absolute
  `http`/`https` URL, Then it exits with a non-zero status and a message naming the variable.
- AC3: Given `ANALYZER_TIMEOUT` (Go duration, e.g. `5s`), Then the API uses it as the analyzer
  timeout; invalid values make the API exit non-zero at startup.

### FR-08 Containers (Must, covers US-05, B-17, C-04)
- AC1: Given Docker, When `docker build` runs in `services/api` and in `services/analyzer`,
  Then each produces an image that starts the service on `$PORT` and serves `GET /healthz`.
- AC2: Both images run as a non-root user and contain no secrets or `.env` files.

### FR-09 Local end-to-end run (Should, covers US-05)
- AC1: Given Docker, When the operator runs the documented single command, Then both services
  start, the API reaches the analyzer, and the documented `curl` example returns the FR-01 AC1 body.

### FR-10 Cloud Run deployment instructions (Must, covers US-06, B-18, B-23)
- AC1: `deploy/cloud-run.md` gives the `gcloud` commands to build and deploy each service
  separately, in the order analyzer → API, with every project-specific value as a placeholder.
- AC2: The instructions make the analyzer not callable from the public internet (mechanism per
  `specs/design.md`, decision Q-01) and pass the analyzer URL to the API via `ANALYZER_URL`.
- AC3: The instructions include a verification step (`curl` against the API URL) and a cleanup step.
- AC4: Nothing in the repository requires GCP credentials to build or test; no command in the
  instructions is executed as part of this project's gates.

## Data model (conceptual)
- **AnalyzeRequest**: `text` — string, required, 1–10,000 code points, not whitespace-only.
- **AnalysisResult**: `word_count` — integer ≥ 0; `character_count` — integer ≥ 0;
  `top_words` — list (0–5) of **WordCount**, ordered by `count` desc then `word` asc.
- **WordCount**: `word` — non-empty lower-case string; `count` — integer ≥ 1.
- **Error**: `error.code` — one of the codes in FR-03 / FR-04 / `not_found` / `internal_error`;
  `error.message` — human-readable English sentence.

## External integrations
- **Analyzer (internal, our own)** — contract in `specs/design.md` §API contracts. From the
  API's side: request `{"text"}`; fields used: `word_count`, `character_count`, `top_words[].word`,
  `top_words[].count`; errors per FR-04; limit: one request per client request, no retries.
- **Google Cloud Run** — only through the deployment instructions (FR-10). `PORT` env and
  `SIGTERM` on shutdown are the platform contract we rely on. Behaviour of the private-service
  mechanism is `UNVERIFIED` until a real deployment.

## Non-functional requirements
- NFR-01 (latency) With the analyzer running locally, `POST /analyze` with a 10,000-character
  text completes in under 500 ms on a developer laptop (excluding container start).
- NFR-02 (privacy) Neither service writes the submitted text to logs. The API logs one line per
  request with method, path, status and duration.
- NFR-03 (graceful shutdown) On `SIGTERM` the API stops accepting new connections and finishes
  in-flight requests for up to 10 s before exiting.
- NFR-04 (dependencies) Go API: standard library only at runtime. Analyzer: FastAPI and an ASGI
  server at runtime; test-only tools are dev dependencies.
- NFR-05 (tests) Every Must AC that is not about deployment has an automated test; tests need
  no network beyond localhost and no GCP access.
- NFR-06 (determinism) The same input always produces byte-identical JSON output.

## Non-goals
NG-01 … NG-07 from `context/shape-notes.md`: no frontend, no persistence, no end-user auth /
API keys / rate limiting, no queue, no actual deployment or credentials now, no linguistic
processing (stemming, stop words, language detection), no batch endpoint. Also: no OpenAPI
generation tooling, no metrics/tracing beyond the request log line.

## Success criteria
- All Must FRs have passing automated tests (except FR-10, which is reviewed by reading).
- Both images build locally and answer `/healthz`.
- `deploy/cloud-run.md` exists and a reviewer can follow it without guessing a value other than
  their own project/region.
- Total implementation effort stays within ~2 hours (B-19).

## Traceability
| Brief | Story | FR | Notes |
|-------|-------|----|-------|
| B-1 | all | all | learning goal; shapes C-05 |
| B-2 | US-01 | FR-01 | |
| B-3 | US-01 | FR-01, FR-03 | C-01 |
| B-4 | US-01, US-06 | FR-02, FR-05, FR-10 | C-02; "private" → Q-01 |
| B-5 | US-01 | FR-01 AC2, FR-04 | C-03 |
| B-6 | US-04, US-06 | FR-06, FR-07, FR-08, FR-10 | C-04 |
| B-7 | US-01 | FR-01 | |
| B-8 | US-02 | FR-03 | |
| B-9 | US-01, US-03 | FR-01 AC2, FR-04 | |
| B-10 | US-01 | FR-02, FR-05 | |
| B-11 | US-01 | FR-01 | |
| B-12 | - | - | this file |
| B-13 | - | - | `specs/design.md` |
| B-14 | - | - | `specs/tasks.md` |
| B-15 | - | - | status line above; approval gate D-08 |
| B-16 | - | - | `specs/tasks.md` item order |
| B-17 | US-05 | FR-08, NFR-05 | |
| B-18 | US-06 | FR-10 | |
| B-19 | - | - | C-05, success criteria |
| B-20 | - | - | C-06, NG-01 … NG-04 |
| B-21 | - | - | C-01, NFR-04 |
| B-22 | - | - | C-07, NFR-04 |
| B-23 | US-05 | FR-10 AC4 | C-08, NG-05 |
| B-24 | - | FR-08 AC2 | C-09 |
| B-25 | - | - | workflow rule in `CLAUDE.md` |

## Open questions
- Q-01 How the analyzer is made private on Cloud Run — see `specs/design.md` §Open questions.
  Affects FR-10 AC2 and possibly one small piece of API code.
- Q-02 Are the limits right: 10,000 characters, 256 KiB body, 10 s analyzer timeout? (A-05, FR-03, FR-04)
- Q-03 Is the word definition in A-03 / FR-02 AC2 acceptable (letters/digits, inner
  apostrophes, `_` splits, digits count as words)?
