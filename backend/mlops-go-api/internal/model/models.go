package model

import "time"

type Prompt struct {
	ID          string
	Title       string
	Description string
	Template    string
	Model       string
}

type Request struct {
	ID         string                 `json:"requestId"`
	PromptID   string                 `json:"promptId"`
	InputJSON  map[string]interface{} `json:"inputJson"`
	Status     string                 `json:"status"`
	ResultText string                 `json:"text,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Usage      map[string]int         `json:"usage,omitempty"`
	LatencyMS  int                    `json:"latencyMs,omitempty"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
}
