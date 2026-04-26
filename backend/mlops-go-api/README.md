# mlops-go-api (backend v0)

Simple Go backend implementing the v0 contract described in the repository README.

---

## High Level Backend Design

### Internal Architecture

```
                        ┌──────────────────────────────────────────────┐
                        │                  Go Backend                   │
                        │                                               │
  HTTP Request          │  ┌────────────┐       ┌────────────────────┐ │
 ──────────────────────►│  │  Routes    │──────►│     Handlers       │ │
                        │  │ /options   │       │  optionsHandler    │ │
  HTTP Response         │  │ /generate  │       │  generateHandler   │ │
 ◄──────────────────────│  │ /requests/ │       │  requestHandler    │ │
                        │  │ /callback/ │       │  callbackHandler   │ │
                        │  │ /health    │       │  healthHandler     │ │
                        │  └────────────┘       └────────┬───────────┘ │
                        │                                │              │
                        │                    ┌───────────▼───────────┐ │
                        │                    │   Generator Service   │ │
                        │                    │  (goroutine per req)  │ │
                        │                    └───────────┬───────────┘ │
                        │                                │              │
                        │              ┌─────────────────┼───────────┐ │
                        │              │                 │           │ │
                        │    ┌─────────▼──────┐  ┌──────▼────────┐  │ │
                        │    │   Repo / Store │  │  LLM Client   │  │ │
                        │    │  (DB or memory)│  │ (Lambda SDK)  │  │ │
                        │    └─────────┬──────┘  └──────┬────────┘  │ │
                        │              │                 │           │ │
                        └──────────────┼─────────────────┼───────────┘ │
                                       │                 │
                              ┌────────▼──────┐  ┌───────▼────────┐
                              │  PostgreSQL   │  │  AWS Lambda    │
                              │  (or memory) │  │                │
                              └───────────────┘  └────────────────┘
```

### Request Lifecycle

```
POST /api/v1/generate
        │
        ├─ Validate input
        ├─ Fetch prompt from DB
        ├─ Insert request row (status = queued)
        ├─ Return 202 + requestId  ◄─── frontend gets this immediately
        │
        └─ Spawn goroutine
                │
                ├─ Set status = running
                ├─ Render template (substitute {{variables}})
                │
                │          BACKEND_BASE_URL set?
                │         ┌──────┴──────┐
               Yes        │            No
                │         │             │
                │    Async invoke   Sync invoke
                │    (Event mode)   (RequestResponse)
                │         │             │
                │         │             ▼
                │         │       AWS Lambda → OpenAI
                │         │             │
                │         │        result returned
                │         │             │
                │    Lambda POSTs  ◄────┘
                │    to /callback
                │         │
                └─────────┴─ Write result to DB (status = done / failed)

GET /api/v1/requests/{id}   ◄── frontend polls this until done
```

### Package Structure

```
cmd/server/
  main.go              Entry point, HTTP server, graceful shutdown

internal/
  api/
    routes.go          Route registration
    handler.go         HTTP handlers + callback endpoint
    response.go        Standard envelope helpers (writeSuccess / writeError)

  service/
    generator.go       Async generation logic, sync/async Lambda path selection

  llm/
    client.go          Client + AsyncClient interfaces, Lambda SDK invocation

  repo/
    store.go           Repo interface, in-memory + Postgres implementations

  db/
    db.go              Postgres connection
    migrate.go         Migration runner
    migrations/        SQL migration files

  model/
    models.go          Prompt and Request structs
```

---

Run locally:

1. Ensure Go is installed (1.18+).
2. From this folder run:

```bash
go mod tidy
go run ./cmd/server
```

Server defaults to `:8080`. Set `PORT` to change.

Notes:
- This implementation uses an in-memory store for prompts and requests (no Postgres required).
- Endpoints:
  - `GET /api/v1/options` — list prompt options
  - `POST /api/v1/generate` — start async generation (returns 202)
  - `GET /api/v1/requests/{id}` — poll request status

Additional notes:
- If you want to run with Postgres, set `DATABASE_URL` and the server will run migrations from `internal/db/migrations` at startup.
- A health endpoint is available at `GET /health` which reports DB connectivity (when configured) and LLM availability.

---

## V2: Lambda-based LLM integration (branch: `lambda-integration`)

In v2 the backend no longer calls OpenAI directly. It invokes an AWS
Lambda function via the AWS SDK for Go v2. All API endpoints, request/
response formats, and database schema remain unchanged.

### What changed in the code

| File | Change |
|---|---|
| `internal/llm/client.go` | Fully replaced — AWS Lambda invoker instead of OpenAI HTTP client |
| `internal/service/generator.go` | Simulated fallback removed; Lambda errors surface as `failed` status |
| `internal/api/handler.go` | Health check reports Lambda config presence instead of OpenAI key |
| `go.mod` / `go.sum` | Added `aws-sdk-go-v2/config` and `aws-sdk-go-v2/service/lambda` |

### Environment variables (v2)

| Variable               | Required | Default     | Description                                      |
|------------------------|----------|-------------|--------------------------------------------------|
| `LAMBDA_FUNCTION_NAME` | **Yes**  | —           | Name or ARN of the Lambda function to invoke     |
| `AWS_REGION`           | **Yes**  | `us-east-1` | AWS region where the Lambda is deployed          |
| `AWS_ACCESS_KEY_ID`    | Yes*     | —           | AWS credential (omit if using an IAM role)       |
| `AWS_SECRET_ACCESS_KEY`| Yes*     | —           | AWS credential (omit if using an IAM role)       |
| `LLM_MODEL`            | No       | `gpt-4o-mini` | Model name forwarded to Lambda in the payload  |
| `PORT`                 | No       | `8080`      | HTTP listen port                                 |
| `DATABASE_URL`         | No       | —           | Postgres connection string                       |

*Not required when the backend runs on an EC2/ECS/EKS instance with an
attached IAM role that grants `lambda:InvokeFunction`.

**Removed in v2:** `OPENAI_API_KEY`, `LLM_TIMEOUT_SECONDS` — these now
live inside the Lambda function itself.

If `LAMBDA_FUNCTION_NAME` is not set the backend logs a fatal error and
exits immediately. There is no silent fallback.

### Lambda invocation contract

The backend is the caller. The Lambda implementation is created manually
in the AWS Console and must honour the following JSON shapes exactly.

#### Input payload (backend → Lambda)

```json
{
  "prompt": "Summarize the following text in 5 bullet points:\nUser text here",
  "model": "gpt-4o-mini",
  "temperature": 0.2
}
```

| Field | Type | Description |
|---|---|---|
| `prompt` | string | Fully rendered prompt (template with variables substituted) |
| `model` | string | OpenAI model identifier, forwarded from `LLM_MODEL` env var |
| `temperature` | float | Sampling temperature; always `0.2` (set by backend) |

#### Output payload (Lambda → backend)

Success:

```json
{
  "text": "• Point one\n• Point two\n...",
  "usage": {
    "inputTokens": 120,
    "outputTokens": 80
  }
}
```

Error — Lambda must return this shape (not raise an unhandled exception):

```json
{
  "text": "",
  "usage": { "inputTokens": 0, "outputTokens": 0 },
  "error": "Human-readable description of what went wrong"
}
```

| Field | Type | Description |
|---|---|---|
| `text` | string | LLM-generated response text |
| `usage.inputTokens` | int | Tokens consumed by the prompt |
| `usage.outputTokens` | int | Tokens in the completion |
| `error` | string | Non-empty only when the Lambda failed; backend marks request as `failed` |

### Lambda environment variables (set in AWS Console)

| Variable | Description |
|---|---|
| `OPENAI_API_KEY` | OpenAI secret key — stays inside Lambda, never in the backend |

Any other model-specific config (e.g. `OPENAI_ORG_ID`) can also live
here without backend changes.

### Required IAM permission

The AWS principal used by the backend must have:

```json
{
  "Effect": "Allow",
  "Action": "lambda:InvokeFunction",
  "Resource": "arn:aws:lambda:<region>:<account-id>:function:<function-name>"
}
```

### Local development with v2

1. Create the Lambda in the AWS Console and note its name/ARN.
2. Set credentials and config in your `.env` or shell:

```bash
LAMBDA_FUNCTION_NAME=my-openai-lambda
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
```

3. Run as before:

```bash
go mod tidy
go run ./cmd/server
```

The backend will invoke the real Lambda on every generation request.
There is no local simulation mode in v2.

### V1 vs V2 comparison

| Aspect | V1 (direct OpenAI) | V2 (Lambda) |
|---|---|---|
| LLM caller | Go backend | AWS Lambda |
| `OPENAI_API_KEY` location | Backend env var | Lambda env var |
| New backend env vars | — | `LAMBDA_FUNCTION_NAME`, `AWS_REGION` |
| API endpoints | unchanged | unchanged |
| Response format | unchanged | unchanged |
| Fallback on missing config | Simulated output | Fatal startup error |
