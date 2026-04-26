package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// Client is the synchronous LLM interface. Lambda invokes and waits for the result.
type Client interface {
	Generate(ctx context.Context, prompt string) (string, map[string]int, error)
}

// AsyncClient extends Client with fire-and-forget Lambda invocation.
// The Lambda calls back the backend via HTTP when done, so the goroutine
// can exit immediately without blocking on the OpenAI response.
type AsyncClient interface {
	Client
	GenerateAsync(ctx context.Context, prompt string, callbackURL string) error
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
	CallbackURL string  `json:"callbackUrl,omitempty"`
}

// lambdaResponse is the JSON payload expected back from the Lambda function
// in synchronous (RequestResponse) invocation mode.
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

// Generate invokes Lambda synchronously (RequestResponse) and returns the result.
// Used when BACKEND_BASE_URL is not set (local dev without a public endpoint).
func (c *lambdaClient) Generate(ctx context.Context, prompt string) (string, map[string]int, error) {
	return c.invoke(ctx, prompt, "", types.InvocationTypeRequestResponse)
}

// GenerateAsync invokes Lambda asynchronously (Event). Lambda will POST the
// result to callbackURL when done. Returns immediately without waiting.
func (c *lambdaClient) GenerateAsync(ctx context.Context, prompt string, callbackURL string) error {
	_, _, err := c.invoke(ctx, prompt, callbackURL, types.InvocationTypeEvent)
	return err
}

func (c *lambdaClient) invoke(ctx context.Context, prompt string, callbackURL string, invType types.InvocationType) (string, map[string]int, error) {
	payload, err := json.Marshal(lambdaRequest{
		Prompt:      prompt,
		Model:       c.model,
		Temperature: 0.2,
		CallbackURL: callbackURL,
	})
	if err != nil {
		return "", nil, fmt.Errorf("lambda: failed to marshal request: %w", err)
	}

	result, err := c.svc.Invoke(ctx, &lambda.InvokeInput{
		FunctionName:   aws.String(c.functionName),
		Payload:        payload,
		InvocationType: invType,
	})
	if err != nil {
		return "", nil, fmt.Errorf("lambda: invocation failed: %w", err)
	}

	// Event invocations return status 202 with no payload — nothing to parse
	if invType == types.InvocationTypeEvent {
		return "", nil, nil
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
