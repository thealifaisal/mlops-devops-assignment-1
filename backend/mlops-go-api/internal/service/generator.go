package service

import (
	"fmt"
	"strings"
	"time"

	"mlops-go-api/internal/repo"
)

type Generator struct {
	store *repo.Store
}

func NewGenerator(s *repo.Store) *Generator {
	return &Generator{store: s}
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

		// simulate LLM call
		start := time.Now()
		time.Sleep(500 * time.Millisecond)
		latency := time.Since(start)

		// produce result (for v0 we echo the rendered template)
		r.ResultText = fmt.Sprintf("Generated (simulated):\n%s", rendered)
		r.Usage = map[string]int{"inputTokens": 10, "outputTokens": 20}
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
