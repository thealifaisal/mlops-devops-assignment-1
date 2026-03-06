package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"mlops-go-api/internal/db"
	"mlops-go-api/internal/llm"
	"mlops-go-api/internal/repo"
	"mlops-go-api/internal/service"
)

var store repo.Repo
var gen *service.Generator

func init() {
	// prefer DATABASE_URL if set
	if url := os.Getenv("DATABASE_URL"); url != "" {
		d, err := db.Connect(url)
		if err == nil {
			// run migrations (best-effort)
			_ = db.Migrate(d, "internal/db/migrations")
			store = repo.NewDBStore(d)
			llmClient := llm.NewFromEnv()
			gen = service.NewGenerator(store, llmClient)
			log.Printf("using Postgres store")
			return
		}
		log.Printf("failed to connect to DB, falling back to memory store: %v", err)
	}
	store = repo.NewStore()
	llmClient := llm.NewFromEnv()
	gen = service.NewGenerator(store, llmClient)
}
func optionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_METHOD", "only GET allowed")
		return
	}
	prompts, err := store.ListPrompts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "could not list prompts")
		return
	}
	data := make([]map[string]string, 0, len(prompts))
	for _, p := range prompts {
		data = append(data, map[string]string{"id": p.ID, "title": p.Title, "description": p.Description})
	}
	writeSuccess(w, http.StatusOK, data)
}

type generateRequest struct {
	OptionID  string                 `json:"optionId"`
	Variables map[string]interface{} `json:"variables"`
}

func generateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_METHOD", "only POST allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "cannot read body")
		return
	}
	var req generateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON")
		return
	}
	if strings.TrimSpace(req.OptionID) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "optionId is required")
		return
	}

	// validate prompt exists
	if _, err := store.GetPrompt(req.OptionID); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "prompt not found")
		return
	}

	rObj, err := store.CreateRequest(req.OptionID, req.Variables)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "could not create request")
		return
	}

	// spawn async processor
	gen.StartProcessing(rObj.ID)

	writeSuccess(w, http.StatusAccepted, map[string]interface{}{"requestId": rObj.ID, "status": rObj.Status})
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_METHOD", "only GET allowed")
		return
	}
	// path is /api/v1/requests/{id}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "missing id")
		return
	}
	id := parts[4]
	log.Printf("GET request id=%q", id)
	reqObj, err := store.GetRequest(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "request not found")
		return
	}

	// Map to response structure expected by frontend
	resp := map[string]interface{}{
		"requestId": reqObj.ID,
		"status":    reqObj.Status,
		"text":      reqObj.ResultText,
		"latencyMs": reqObj.LatencyMS,
		"usage":     reqObj.Usage,
	}
	if reqObj.Status == "failed" {
		resp["error"] = reqObj.Error
	}

	// Log simple access
	log.Printf("GET request %s status=%s", id, reqObj.Status)

	writeSuccess(w, http.StatusOK, resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_METHOD", "only GET allowed")
		return
	}

	status := "ok"
	res := map[string]interface{}{"status": status}

	// DB check if configured
	if url := os.Getenv("DATABASE_URL"); url != "" {
		d, err := db.Connect(url)
		if err != nil {
			res["db"] = "down"
			res["status"] = "degraded"
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := d.PingContext(ctx); err != nil {
				res["db"] = "down"
				res["status"] = "degraded"
			} else {
				res["db"] = "ok"
			}
			_ = d.Close()
		}
	} else {
		res["db"] = "memory"
	}

	// LLM presence
	if llm.NewFromEnv() != nil {
		res["llm"] = "available"
	} else {
		res["llm"] = "unavailable"
	}

	writeSuccess(w, http.StatusOK, res)
}

// SetDeps allows tests to inject a repo and generator.
func SetDeps(s repo.Repo, g *service.Generator) {
	store = s
	gen = g
}
