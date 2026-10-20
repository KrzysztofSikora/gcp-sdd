# Text Analysis API (gcp-sdd)

A learning project: a public **Go** REST API (`POST /analyze`) that validates text and calls a
private **Python FastAPI** analysis service over HTTP; two independently deployable **Cloud Run**
services. The brief is `docs/brief.md`; the specs live in `specs/`, the workflow's other
artifacts in `context/`.

## Process

Spec-driven workflow in `.claude/skills/`, described in `docs/workflow.md` (copied from
MeshCore Guardian; project artifacts are mapped to `specs/`, see *Setup in this repository* there).

- Project steps are interactive and run once; the user approves each:
  `/shape` (`context/shape-notes.md`) → `/prd` (`specs/requirements.md`) →
  `/tech-stack` (`specs/design.md`) → `/roadmap` (`specs/tasks.md`) → `/test-plan`
  (`context/test-plan.md`).
- **No implementation before the user approves `specs/`** (brief B-15). The status line at the
  top of each spec says whether it is approved.
- Per-change steps are autonomous and run from `.claude/workflow/ship <task ID>` (or `/ship`):
  plan → plan-review → implement (one session per phase) → mutation → review → fix-review → PR.
  They never ask; they record decisions in the plan and the PR collects them.
  `.claude/workflow/address-pr <n>` applies the user's PR decisions.
- Rules for autonomous runs: `.claude/workflow/autonomy.md`.
- Do not start coding a task outside that loop, unless the user asks for it explicitly (then
  note it in `specs/tasks.md`, as for the first implementation on 2026-10-20).

## Artifacts

| File | Owner |
|------|-------|
| `docs/brief.md` | the user (verbatim brief, B-n IDs) |
| `context/shape-notes.md`, `specs/requirements.md`, `specs/design.md`, `specs/tasks.md`, `context/test-plan.md` | project skills |
| `context/changes/<id>/plan.md` | `/plan` (decisions, phases, human checks, deviations) |
| `context/changes/<id>/{plan-review,mutation,review}.md` | per-change skills |
| `context/changes/<id>/pr-body.md`, `context/changes/<id>/evidence/` | `/pr-body`, `/implement` |
| `context/lessons.md` | rules from reviews; binding for planning, implementing and reviewing |

Artifacts are the source of truth. If code and an artifact disagree, raise it (in an autonomous
run: a `[REVIEW]` deviation); don't silently pick one. Do not change approved specs to make
code or tests pass.

## Rules

- No secrets in code, commits, logs or answers. Local config comes from `.env` (template:
  `.env.example`); the project needs no secrets.
- **No GCP access**: agents never run `gcloud`/`gsutil`/`bq` against a project, never deploy and
  never ask for credentials (brief B-23). Deployment is documented in `deploy/cloud-run.md` and
  done by the person.
- No AI attribution anywhere in the repo: no `Co-Authored-By`, `Claude-Session` or "Generated with"
  lines in commits, PR descriptions or issues.
- One Conventional Commit per implemented phase, scope = change-id: `feat(<change-id>): ...`.
- Commits reference their GitHub issue with `Refs #<n>` when the task has one; the PR closes it.
- Go API (`services/api`): standard library only at runtime. Layers: `main` → `internal/httpapi`
  → `internal/analyzer` (through an interface); `internal/config` only from `main`.
- Analyzer (`services/analyzer`): `app/analysis.py` is pure (no FastAPI imports); `app/main.py`
  is the HTTP layer. Runtime deps: FastAPI + Uvicorn only, pinned exactly.
- Never log the submitted text. Errors from the Go API are JSON `{"error": {"code", "message"}}`
  with the codes in `specs/design.md`.
- Keep the MVP small (1–2 h, B-19): no frontend, database, auth, queue (B-20). Anything else goes
  to *Later* in `specs/tasks.md`.

## Gates

The autonomous skills run exactly these commands. Keep them current. `/implement` runs the
fast ones per phase; `/review` runs all of them.

| Gate | Command | Needs |
|------|---------|-------|
| lint | `make lint` | Go 1.24, Python 3.13 venv with dev deps |
| unit tests | `make test` | same |
| images | `make docker-build` | Docker |
| mutation (one module) | decided in `/test-plan`; until then ship with `--no-mutation` | - |

The Makefile is created by F-01 (repository skeleton). Until then the gates fail.

## Commands

| Task | Command |
|------|---------|
| Analyzer setup | `cd services/analyzer && python -m venv .venv && . .venv/bin/activate && pip install -e '.[dev]'` |
| Run analyzer | `cd services/analyzer && PORT=8081 uvicorn app.main:app --port 8081` |
| Run API | `cd services/api && ANALYZER_URL=http://localhost:8081 go run .` |
| Both in Docker | `docker compose up --build` |
| Try it | `curl -s localhost:8080/analyze -H 'Content-Type: application/json' -d '{"text":"The cat and the hat."}'` |
