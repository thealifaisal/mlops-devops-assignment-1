package unit_tests

import (
    "context"
    "testing"
    "time"

    "mlops-go-api/internal/repo"
    "mlops-go-api/internal/service"
)

type fakeLLM struct{}

func (f *fakeLLM) Generate(_ context.Context, prompt string) (string, map[string]int, error) {
    return "FAKE: " + prompt, map[string]int{"inputTokens": 1, "outputTokens": 2}, nil
}

// simple test to ensure generator calls LLM and updates request
func TestGeneratorProcessesRequest(t *testing.T) {
    store := repo.NewStore()
    fake := &fakeLLM{}
    gen := service.NewGenerator(store, fake)

    // create request
    r, err := store.CreateRequest("summarize_v1", map[string]interface{}{"text": "hello"})
    if err != nil {
        t.Fatal(err)
    }

    gen.StartProcessing(r.ID)

    // wait for completion
    deadline := time.Now().Add(3 * time.Second)
    for time.Now().Before(deadline) {
        rr, err := store.GetRequest(r.ID)
        if err != nil {
            t.Fatal(err)
        }
        if rr.Status == "done" {
            if rr.ResultText == "" {
                t.Fatalf("expected result text, got empty")
            }
            return
        }
        time.Sleep(50 * time.Millisecond)
    }
    t.Fatalf("request did not complete in time")
}
