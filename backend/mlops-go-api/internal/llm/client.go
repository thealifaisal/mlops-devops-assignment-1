package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// Client is the LLM client interface used by the service. Tests can implement this.
type Client interface {
	Generate(ctx context.Context, prompt string) (string, map[string]int, error)
}

type lambdaClient struct {
	svc          *lambda.Client
	functionName string
	model        string
}

// lambdaRequest is the JSON payload sent to the Lambda function.
type lambdaRequest struct {
	Prompt      string  `json:"prompt"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
}

// lambdaResponse is the JSON payload expected back from the Lambda function.
type lambdaResponse struct {
	Text  string         `json:"text"`
	Usage map[string]int `json:"usage"`
	Error string         `json:"error,omitempty"`
}

// NewFromEnv returns a Client configured from environment variables.
// Returns nil if LAMBDA_FUNCTION_NAME is not set; the caller must treat
// nil as a fatal misconfiguration and surface a clear error.
func NewFromEnv() Client {
	functionName := os.Getenv("LAMBDA_FUNCTION_NAME")
	if functionName == "" {
		log.Printf("llm: LAMBDA_FUNCTION_NAME is not set — LLM client unavailable")
		return nil
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("llm: failed to load AWS config: %v", err)
	}

	return &lambdaClient{
		svc:          lambda.NewFromConfig(cfg),
		functionName: functionName,
		model:        model,
	}
}

// Generate invokes the configured Lambda function and returns the generated text and token usage.
func (c *lambdaClient) Generate(ctx context.Context, prompt string) (string, map[string]int, error) {
	payload, err := json.Marshal(lambdaRequest{
		Prompt:      prompt,
		Model:       c.model,
		Temperature: 0.2,
	})
	if err != nil {
		return "", nil, fmt.Errorf("lambda: failed to marshal request: %w", err)
	}

	result, err := c.svc.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: &c.functionName,
		Payload:      payload,
	})
	if err != nil {
		return "", nil, fmt.Errorf("lambda: invocation failed: %w", err)
	}

	if result.FunctionError != nil {
		return "", nil, fmt.Errorf("lambda: function error=%s payload=%s", *result.FunctionError, string(result.Payload))
	}

	var resp lambdaResponse
	if err := json.Unmarshal(result.Payload, &resp); err != nil {
		return "", nil, fmt.Errorf("lambda: failed to parse response: %w", err)
	}

	if resp.Error != "" {
		return "", nil, fmt.Errorf("lambda: LLM error: %s", resp.Error)
	}

	return resp.Text, resp.Usage, nil
}
