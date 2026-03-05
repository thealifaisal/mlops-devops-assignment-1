package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"mlops-go-api/internal/repo"
	"mlops-go-api/internal/service"
)

var store = repo.NewStore()
var gen = service.NewGenerator(store)

func optionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_METHOD", "only GET allowed")
		return
	}
	prompts := store.ListPrompts()
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
		resp["error_message"] = reqObj.Error
	}

	// Log simple access
	log.Printf("GET request %s status=%s", id, reqObj.Status)

	writeSuccess(w, http.StatusOK, resp)
}
