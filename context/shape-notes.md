# Shape notes: Text Analysis API

Brief: `docs/brief.md` (B-1 … B-25).

## Brief summary
A two-service learning project. A public Go REST API accepts text on `POST /analyze`,
validates it, and forwards it over HTTP to a private Python FastAPI service. The Python
service computes word count, character count and the five most frequent words, and the Go API
returns that result as JSON. Each service ships as its own container and is deployable to Cloud
Run on its own. The point is learning Go, Python and Cloud Run with a spec-driven, AI-assisted
workflow, so the scope stays at a 1–2 hour MVP with nothing beyond the two services.

## Decisions
| ID | Topic | Decision | Reason | Source (brief / user / default) |
|----|-------|----------|--------|---------------------------------|
| D-01 | Languages and frameworks | Go for the public API, Python + FastAPI for the analyzer | Mandated | brief B-3, B-4 |
| D-02 | Inter-service protocol | Synchronous HTTP/1.1 with JSON bodies | Mandated; simplest | brief B-5 |
| D-03 | Deployment unit | Two containers, two Cloud Run services, deployable independently | Mandated | brief B-6 |
| D-04 | Who validates | Go is the validation authority for client input; the analyzer re-checks the same limits defensively | Go must validate (B-8); the analyzer must not trust a caller it cannot see | brief B-8, default |
| D-05 | Public response shape | The Go API returns the analysis fields with the same names the analyzer uses, but through its own type (not a raw proxy of the analyzer's bytes) | Lets each service change on its own; Go can check the analyzer's answer | default |
| D-06 | Error format | Every non-2xx response from the Go API is JSON `{"error": {"code", "message"}}` with a stable `code` | Clients branch on a code, not on prose | default |
| D-07 | Specs location | `specs/requirements.md`, `specs/design.md`, `specs/tasks.md` take the roles of the workflow's PRD, tech stack and roadmap | Mandated file names | brief B-12 … B-14 |
| D-08 | Approval gate | No implementation before the user approves `specs/` | Mandated | brief B-15 |

## Assumptions
- A-01 Text is UTF-8. "Characters" means Unicode code points, not bytes (cheap to change: one function).
- A-02 Words are case-insensitive; top words are reported in lower case.
- A-03 A word is a run of Unicode letters or digits, optionally joined by an apostrophe
  (`don't`, `l'été`). Punctuation, whitespace and `_` separate words. No stop-word filtering.
- A-04 Ties in the top 5 are broken alphabetically, so results are deterministic and testable.
- A-05 Maximum text length 10,000 characters; anything longer is rejected, not truncated.
- A-06 The analyzer is called with a timeout and without retries; a failure is reported to the client.
- A-07 Both services listen on the port in `$PORT` (Cloud Run contract).
- A-08 Neither service logs the submitted text.

## Non-goals
- NG-01 No frontend (B-20).
- NG-02 No database or any persistence; nothing is stored between requests (B-20).
- NG-03 No end-user authentication, API keys or rate limiting (B-20).
- NG-04 No message queue or async processing (B-20).
- NG-05 No actual deployment, no GCP project, no credentials in this phase (B-23).
- NG-06 No language detection, stemming, lemmatisation or stop words.
- NG-07 No batch endpoint; one text per request.

## Deferred questions
- Q-01 How is the analyzer kept private on Cloud Run (IAM invoker + ID token vs internal
  ingress + VPC egress)? Does not block the requirements; it is a design and deployment choice
  and is asked at the design approval (B-15). See `specs/design.md` §Open questions.

## Risks spotted
- R-01 Cloud Run cold start of the Python service may exceed a short client timeout
  (medium / low). Mitigation: timeout is configurable; deployment notes mention min instances.
- R-02 Scope creep past 1–2 hours (medium / medium). Mitigation: Should/Could items in
  `specs/tasks.md` are cut first.
- R-03 "Private" may be read as requiring service-to-service authentication, which brushes
  against B-20 (medium / medium). Mitigation: asked explicitly as Q-01.
