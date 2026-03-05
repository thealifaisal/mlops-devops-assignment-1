# mlops-go-api (backend v0)

Simple Go backend implementing the v0 contract described in the repository README.

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
