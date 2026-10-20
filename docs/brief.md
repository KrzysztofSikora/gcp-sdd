# Brief: Text Analysis API

The brief as given by the user, verbatim. `context/shape-notes.md` and
`specs/requirements.md` cite its lines as `B-<n>`.

> Build a small project using Spec-Driven Development.

| ID | Requirement |
|----|-------------|
| B-1 | Goal: learn Go, Python and Google Cloud Run while practicing AI-assisted software engineering. |
| B-2 | Project: Text Analysis API. |
| B-3 | Architecture: Go REST API (public). |
| B-4 | Architecture: Python FastAPI analysis service (private). |
| B-5 | Architecture: HTTP communication between services. |
| B-6 | Architecture: two independently deployable Cloud Run services. |
| B-7 | Feature: `POST /analyze` accepts text. |
| B-8 | Feature: Go validates the request. |
| B-9 | Feature: Go calls the Python service. |
| B-10 | Feature: Python returns word count, character count and top 5 words. |
| B-11 | Feature: Go returns the result as JSON. |
| B-12 | Process: create `specs/requirements.md` with acceptance criteria. |
| B-13 | Process: create `specs/design.md` with architecture and API contracts. |
| B-14 | Process: create `specs/tasks.md` with small implementation tasks. |
| B-15 | Process: stop and ask the user to approve the design. |
| B-16 | Process: implement tasks incrementally. |
| B-17 | Process: add unit tests and Dockerfiles. |
| B-18 | Process: add Cloud Run deployment instructions. |
| B-19 | Constraint: keep the MVP achievable in 1–2 hours. |
| B-20 | Constraint: no frontend, database, authentication or message queue. |
| B-21 | Constraint: prefer the Go standard library. |
| B-22 | Constraint: keep dependencies minimal. |
| B-23 | Constraint: do not deploy or require GCP credentials yet. |
| B-24 | Constraint: do not commit secrets. |
| B-25 | Constraint: follow the approved specifications. |
