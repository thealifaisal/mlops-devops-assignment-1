package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"mlops-go-api/internal/llm"
	"mlops-go-api/internal/repo"
)

type Generator struct {
	store repo.Repo
	llm   llm.Client
}

func NewGenerator(s repo.Repo, l llm.Client) *Generator {
	return &Generator{store: s, llm: l}
}

func (g *Generator) StartProcessing(id string) {
	go func() {
		// mark running
		r, err := g.store.GetRequest(id)
		if err != nil {
			return
		}
		r.Status = "running"
		r.StartedAt = time.Now()
		_ = g.store.UpdateRequest(r)

		// render template
		prompt, err := g.store.GetPrompt(r.PromptID)
		if err != nil {
			r.Status = "failed"
			r.Error = "prompt not found"
			_ = g.store.UpdateRequest(r)
			return
		}

		rendered := renderTemplate(prompt.Template, r.InputJSON)

		// nil client means LAMBDA_FUNCTION_NAME was not set
		if g.llm == nil {
			log.Printf("generator: LAMBDA_FUNCTION_NAME is not configured — cannot process request %s", id)
			r.Status = "failed"
			r.Error = "LLM_ERROR: LAMBDA_FUNCTION_NAME is not configured"
			r.FinishedAt = time.Now()
			_ = g.store.UpdateRequest(r)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
		defer cancel()

		// Async path: if BACKEND_BASE_URL is set and the client supports async invocation,
		// fire Lambda as an Event and let it POST the result back via the callback endpoint.
		// The goroutine exits immediately — no blocking on OpenAI.
		if baseURL := os.Getenv("BACKEND_BASE_URL"); baseURL != "" {
			if ac, ok := g.llm.(llm.AsyncClient); ok {
				callbackURL := strings.TrimRight(baseURL, "/") + "/api/v1/internal/callback/" + r.ID
				log.Printf("generator: async invocation for request %s, callback=%s", id, callbackURL)
				if err := ac.GenerateAsync(ctx, rendered, callbackURL); err != nil {
					log.Printf("generator: GenerateAsync error=%v", err)
					r.Status = "failed"
					r.Error = err.Error()
					r.FinishedAt = time.Now()
					_ = g.store.UpdateRequest(r)
				}
				// Lambda will call back to update the request
				return
			}
		}

		// Sync path: invoke Lambda and wait for the result (used when BACKEND_BASE_URL is not set).
		log.Printf("generator: sync invocation for request %s", id)
		start := time.Now()
		text, usage, err := g.llm.Generate(ctx, rendered)
		if err != nil {
			log.Printf("generator: llm.Generate error=%v", err)
			r.Status = "failed"
			r.Error = err.Error()
			r.FinishedAt = time.Now()
			_ = g.store.UpdateRequest(r)
			return
		}
		log.Printf("generator: llm.Generate succeeded, tokens=%v", usage)
		latency := time.Since(start)

		r.ResultText = text
		r.Usage = usage
		r.LatencyMS = int(latency / time.Millisecond)
		r.Status = "done"
		r.FinishedAt = time.Now()
		_ = g.store.UpdateRequest(r)
	}()
}

func renderTemplate(tmpl string, vars map[string]interface{}) string {
	out := tmpl
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		out = strings.ReplaceAll(out, placeholder, fmt.Sprint(v))
	}
	// simple cleanup: remove any unreplaced placeholders
	out = strings.ReplaceAll(out, "{{", "")
	out = strings.ReplaceAll(out, "}}", "")
	return out
}
