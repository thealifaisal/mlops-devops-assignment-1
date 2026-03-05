# LLM Prompt Runner -- Backend v0

This repository contains the **v0 implementation plan** for the backend
service used by the React frontend. The system allows a user to select a
prompt option, submit text, and asynchronously generate an LLM response.

The frontend already expects the following API behavior, so the backend
must strictly follow this contract.

------------------------------------------------------------------------

# System Overview

Architecture (v0):

User → React Frontend → Go Backend API → Postgres → LLM API

Important characteristics:

-   Backend is a **single Go service (monolith)**
-   Async processing is done **in-process using goroutines**
-   **Postgres** stores prompts and request states
-   Frontend **polls** for request completion
-   API responses follow a **standard envelope format**

Future assignments may evolve this into worker queues, streaming, etc.

------------------------------------------------------------------------

# API Response Format (Mandatory)

All responses must follow this envelope structure.

Success:

{ "success": true, "data": {...} }

Failure:

{ "success": false, "error": { "code": "ERROR_CODE", "message": "Human
readable message" } }

The frontend **expects JSON responses**. Always return:

Content-Type: application/json

------------------------------------------------------------------------

# API Endpoints

Base path used by frontend:

/api/v1/

## 1. List Prompt Options

GET /api/v1/options

Response:

{ "success": true, "data": \[ { "id": "summarize_v1", "title":
"Summarize", "description": "Summarize text into bullet points" } \] }

The frontend uses this to populate the dropdown.

------------------------------------------------------------------------

## 2. Start Generation (Async)

POST /api/v1/generate

Request:

{ "optionId": "summarize_v1", "variables": { "text": "Some text" } }

Response (202):

{ "success": true, "data": { "requestId": "uuid", "status": "queued" } }

Backend must:

1.  Validate input
2.  Fetch prompt template
3.  Insert row into requests table
4.  Spawn async processor
5.  Return requestId

------------------------------------------------------------------------

## 3. Get Request Status

GET /api/v1/requests/{id}

Response:

{ "success": true, "data": { "requestId": "uuid", "status": "queued \|
running \| done \| failed", "text": "...", "latencyMs": 1234, "usage": {
"inputTokens": 120, "outputTokens": 80 } } }

Frontend polls this endpoint until status = done or failed.

------------------------------------------------------------------------

# Database Schema

Postgres database.

## prompts table

Stores available prompt options.

Columns:

id TEXT PRIMARY KEY title TEXT NOT NULL description TEXT template TEXT
NOT NULL model TEXT NOT NULL temperature DOUBLE PRECISION DEFAULT 0.2
max_tokens INT DEFAULT 512 is_active BOOLEAN DEFAULT TRUE created_at
TIMESTAMP DEFAULT now() updated_at TIMESTAMP DEFAULT now()

Example row:

id: summarize_v1

template: Summarize the following text in 5 bullet points:

{{text}}

------------------------------------------------------------------------

## requests table

Tracks async generation state.

Columns:

id UUID PRIMARY KEY prompt_id TEXT REFERENCES prompts(id) input_json
JSONB status TEXT result_text TEXT error_message TEXT usage_json JSONB
latency_ms INT created_at TIMESTAMP DEFAULT now() updated_at TIMESTAMP
DEFAULT now() started_at TIMESTAMP finished_at TIMESTAMP

Status values:

queued running done failed

------------------------------------------------------------------------

# Backend Processing Flow

POST /generate:

1.  Validate request
2.  Fetch prompt template
3.  Insert request row with status=queued
4.  Return requestId
5.  Spawn goroutine

Async goroutine:

1.  Update status → running
2.  Render template
3.  Call LLM API
4.  Store result
5.  Update status → done
6.  If error → status → failed

------------------------------------------------------------------------

# Prompt Template Rendering

Templates may contain variables like:

{{text}}

Backend should replace placeholders using values from variables map.

For v0 only text is required, but implementation should support generic
keys.

------------------------------------------------------------------------

# Environment Variables

Backend must support:

PORT=8080

DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=disable

OPENAI_API_KEY=your_api_key

LLM_MODEL=gpt-4o-mini

LLM_TIMEOUT_SECONDS=120

------------------------------------------------------------------------

# Suggested Go Project Structure

backend/ cmd/server/main.go internal/ api/ routes.go handler.go
response.go dto.go db/ db.go migrate.go repo/ prompts_repo.go
requests_repo.go service/ generator_service.go processor.go llm/
client.go openai_client.go migrations/ 001_init.sql

------------------------------------------------------------------------

# Running the System (Local Development)

1.  Start Postgres

2.  Run migrations

3.  Start backend server

go run cmd/server/main.go

Server runs on:

http://localhost:8080

4.  Start frontend

Frontend will call:

/api/v1/...

Ensure reverse proxy or dev proxy maps /api → backend.

------------------------------------------------------------------------

# Error Codes

Use these standardized codes:

INVALID_JSON VALIDATION_ERROR NOT_FOUND DB_ERROR LLM_ERROR LLM_TIMEOUT
MISSING_API_KEY

------------------------------------------------------------------------

# v0 Limitations

This version intentionally keeps architecture simple.

Limitations:

-   Async processing uses goroutines
-   If server restarts during generation, request may be lost
-   No distributed queue yet

Future versions may introduce:

-   Redis queue
-   worker service
-   streaming responses
-   observability

------------------------------------------------------------------------

# Definition of Done

The system is complete when:

1.  GET /options returns at least one option
2.  POST /generate returns requestId
3.  Polling /requests/{id} progresses queued → running → done
4.  Frontend displays generated output
5.  All responses follow API envelope format
