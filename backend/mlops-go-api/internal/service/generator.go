package service

import (
	"context"
	"fmt"
	"log"
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

		// invoke Lambda-backed LLM — nil client means LAMBDA_FUNCTION_NAME was not set
		if g.llm == nil {
			log.Printf("generator: LAMBDA_FUNCTION_NAME is not configured — cannot process request %s", id)
			r.Status = "failed"
			r.Error = "LLM_ERROR: LAMBDA_FUNCTION_NAME is not configured"
			r.FinishedAt = time.Now()
			_ = g.store.UpdateRequest(r)
			return
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
		defer cancel()
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
