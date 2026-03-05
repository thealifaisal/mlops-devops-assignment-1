package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mlops-go-api/internal/llm"
	"mlops-go-api/internal/repo"
)

type Generator struct {
	store repo.Repo
	llm   *llm.Client
}

func NewGenerator(s repo.Repo) *Generator {
	return &Generator{store: s, llm: llm.NewFromEnv()}
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

		// call LLM (fallback to simulated if no API key)
		start := time.Now()
		var text string
		var usage map[string]int
		if g.llm != nil {
			ctx, cancel := context.WithTimeout(context.Background(),  time.Second*120)
			defer cancel()
			t, u, err := g.llm.Generate(ctx, rendered)
			if err == nil {
				text = t
				usage = u
			} else {
				// fallback
				text = fmt.Sprintf("Generated (simulated-fallback):\n%s", rendered)
				usage = map[string]int{"inputTokens": 0, "outputTokens": 0}
			}
		} else {
			text = fmt.Sprintf("Generated (simulated):\n%s", rendered)
			usage = map[string]int{"inputTokens": 10, "outputTokens": 20}
		}
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
