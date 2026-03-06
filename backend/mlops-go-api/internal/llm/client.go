package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Client is the LLM client interface used by the service. Tests can implement this.
type Client interface {
	Generate(ctx context.Context, prompt string) (string, map[string]int, error)
}

type httpClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewFromEnv returns a Client configured from environment variables.
// If OPENAI_API_KEY is not set, it returns nil (caller can provide a simulated client).
func NewFromEnv() Client {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	to := 120
	if s := os.Getenv("LLM_TIMEOUT_SECONDS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			to = v
		}
	}
	return &httpClient{apiKey: apiKey, model: model, client: &http.Client{Timeout: time.Duration(to) * time.Second}}
}

// Generate sends the prompt to OpenAI Chat Completions and returns text and usage map.
func (c *httpClient) Generate(ctx context.Context, prompt string) (string, map[string]int, error) {
	if c.apiKey == "" {
		return "", nil, fmt.Errorf("missing OPENAI_API_KEY")
	}

	body := map[string]interface{}{
		"model":       c.model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0.2,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// read and log response body for diagnostics
		b, _ := io.ReadAll(resp.Body)
		log.Printf("llm http error status=%d body=%s", resp.StatusCode, string(b))
		var errBody map[string]interface{}
		if err := json.Unmarshal(b, &errBody); err == nil {
			return "", nil, fmt.Errorf("llm error: status=%d body=%v", resp.StatusCode, errBody)
		}
		return "", nil, fmt.Errorf("llm error: status=%d body=%s", resp.StatusCode, string(b))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage map[string]int `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", nil, err
	}
	text := ""
	if len(out.Choices) > 0 {
		text = out.Choices[0].Message.Content
	}
	return text, out.Usage, nil
}
