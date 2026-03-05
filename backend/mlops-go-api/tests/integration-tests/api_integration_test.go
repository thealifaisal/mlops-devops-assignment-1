package integration_tests

import (
    "bytes"
    "encoding/json"
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "mlops-go-api/internal/api"
    "mlops-go-api/internal/repo"
    "mlops-go-api/internal/service"
)

type fakeLLM struct{}

func (f *fakeLLM) Generate(_ context.Context, prompt string) (string, map[string]int, error) {
    return "FAKE: " + prompt, map[string]int{"inputTokens": 1, "outputTokens": 2}, nil
}

func TestAPIEndToEnd(t *testing.T) {
    store := repo.NewStore()
    fake := &fakeLLM{}
    gen := service.NewGenerator(store, fake)
    api.SetDeps(store, gen)

    mux := http.NewServeMux()
    api.RegisterRoutes(mux)
    srv := httptest.NewServer(mux)
    defer srv.Close()

    // GET options
    res, err := http.Get(srv.URL + "/api/v1/options")
    if err != nil { t.Fatal(err) }
    if res.StatusCode != 200 { t.Fatalf("expected 200, got %d", res.StatusCode) }

    // POST generate
    body := map[string]interface{}{"optionId":"summarize_v1","variables":map[string]interface{}{"text":"hello"}}
    b, _ := json.Marshal(body)
    resp, err := http.Post(srv.URL+"/api/v1/generate","application/json", bytes.NewReader(b))
    if err != nil { t.Fatal(err) }
    if resp.StatusCode != http.StatusAccepted {
        t.Fatalf("expected 202, got %d", resp.StatusCode)
    }
    var env map[string]interface{}
    _ = json.NewDecoder(resp.Body).Decode(&env)
    data := env["data"].(map[string]interface{})
    rid := data["requestId"].(string)

    // poll
    deadline := time.Now().Add(5 * time.Second)
    for time.Now().Before(deadline) {
        rres, err := http.Get(srv.URL + "/api/v1/requests/" + rid)
        if err != nil { t.Fatal(err) }
        var er map[string]interface{}
        _ = json.NewDecoder(rres.Body).Decode(&er)
        d := er["data"].(map[string]interface{})
        if d["status"].(string) == "done" {
            if d["text"].(string) == "" {
                t.Fatalf("expected text")
            }
            return
        }
        time.Sleep(100 * time.Millisecond)
    }
    t.Fatalf("request did not finish in time")
}
