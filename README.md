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
| `docs/workflow.md` | the spec-driven, AI-assisted workflow |

Status: specifications drafted, awaiting approval. No code yet.
