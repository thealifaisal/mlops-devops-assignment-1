import { useEffect, useMemo, useRef, useState } from "react";

type APIResponse<T> = {
  success: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
  };
};

type Option = {
  id: string;
  title: string;
  description?: string;
};

type GenerateResponse = {
  requestId: string;
  status: "queued" | "running" | "done" | "failed";
};

type RequestResponse = {
  requestId: string;
  status: "queued" | "running" | "done" | "failed";
  text?: string;
  error?: string;
  usage?: {
    inputTokens?: number;
    outputTokens?: number;
  };
  latencyMs?: number;
};

const API_BASE = "/api";

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export default function App() {
  const [options, setOptions] = useState<Option[]>([]);
  const [selectedOptionId, setSelectedOptionId] = useState<string>("");
  const [text, setText] = useState("");

  const [requestId, setRequestId] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [output, setOutput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [meta, setMeta] = useState<any>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [serverAvailable, setServerAvailable] = useState<boolean>(true);
  const [isCheckingServer, setIsCheckingServer] = useState(false);

  const pollAbortRef = useRef<AbortController | null>(null);

  const canSubmit = useMemo(() => {
    return selectedOptionId && text.trim() !== "" && !isSubmitting && serverAvailable;
  }, [selectedOptionId, text, isSubmitting, serverAvailable]);

  // ------------------------
  // Load options
  // ------------------------
  async function loadOptions() {
    try {
      const res = await fetch(`${API_BASE}/v1/options`);
      
      // Check if response is JSON
      const contentType = res.headers.get("content-type");
      if (!contentType || !contentType.includes("application/json")) {
        throw new Error("Server is not responding with valid JSON. The backend may be down.");
      }

      const json: APIResponse<Option[]> = await res.json();

      if (!json.success) {
        throw new Error(json.error?.message || "Failed to load options");
      }

      setOptions(json.data || []);
      if (json.data && json.data.length > 0) {
        setSelectedOptionId(json.data[0].id);
      }
      setServerAvailable(true);
      setError(null);
    } catch (err: any) {
      console.error("Failed to load options:", err.message);
      setServerAvailable(false);
      setOptions([]);
      setError(`Unable to connect to server: ${err.message}`);
    }
  }

  useEffect(() => {
    loadOptions();
  }, []);

  // ------------------------
  // Retry connection
  // ------------------------
  async function retryConnection() {
    setIsCheckingServer(true);
    setError(null);
    await loadOptions();
    setIsCheckingServer(false);
  }

  // ------------------------
  // Polling
  // ------------------------
  async function pollRequest(id: string) {
    if (pollAbortRef.current) pollAbortRef.current.abort();

    const controller = new AbortController();
    pollAbortRef.current = controller;

    let attempt = 0;

    while (!controller.signal.aborted) {
      try {
        const res = await fetch(`${API_BASE}/v1/requests/${id}`, {
          signal: controller.signal,
        });

        const contentType = res.headers.get("content-type");
        if (!contentType || !contentType.includes("application/json")) {
          throw new Error("Server returned non-JSON response");
        }

        const json: APIResponse<RequestResponse> = await res.json();

        if (!json.success) {
          throw new Error(json.error?.message || "Polling failed");
        }

        const data = json.data!;
        setStatus(data.status);
        setError(data.error || null);

        if (data.status === "done") {
          setOutput(data.text || "");
          setMeta({
            latencyMs: data.latencyMs,
            usage: data.usage,
          });
          return;
        }

        if (data.status === "failed") {
          setOutput("");
          return;
        }

        // backoff
        attempt++;
        let delay = 1000;
        if (attempt > 5) delay = 2000;
        if (attempt > 10) delay = 4000;

        await sleep(delay);
      } catch (err: any) {
        if (controller.signal.aborted) return;

        setError(err.message);
        await sleep(2000);
      }
    }
  }

  // ------------------------
  // Submit
  // ------------------------
  async function onSubmit() {
    if (!canSubmit) return;

    setIsSubmitting(true);
    setError(null);
    setOutput("");
    setMeta({});
    setStatus(null);
    setRequestId(null);

    if (pollAbortRef.current) pollAbortRef.current.abort();

    try {
      const res = await fetch(`${API_BASE}/v1/generate`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          optionId: selectedOptionId,
          variables: { text },
        }),
      });

      const contentType = res.headers.get("content-type");
      if (!contentType || !contentType.includes("application/json")) {
        throw new Error("Server is not responding properly. Please check if the backend is running.");
      }

      const json: APIResponse<GenerateResponse> = await res.json();

      if (!json.success) {
        throw new Error(json.error?.message || "Generate failed");
      }

      const data = json.data!;
      setRequestId(data.requestId);
      setStatus(data.status);

      pollRequest(data.requestId);
    } catch (err: any) {
      setError(err.message);
      setServerAvailable(false);
    } finally {
      setIsSubmitting(false);
    }
  }

  // ------------------------
  // Reset
  // ------------------------
  function reset() {
    if (pollAbortRef.current) pollAbortRef.current.abort();
    setRequestId(null);
    setStatus(null);
    setOutput("");
    setError(null);
    setMeta({});
  }

  // ------------------------
  // UI
  // ------------------------
  return (
    <div style={{ maxWidth: 900, margin: "30px auto", fontFamily: "Arial" }}>
      <h1>LLM Prompt Runner</h1>
      
      {!serverAvailable && (
        <div style={{ 
          background: "#f8d7da", 
          border: "1px solid #f5c6cb", 
          color: "#721c24",
          padding: "15px", 
          borderRadius: "4px",
          marginBottom: "20px"
        }}>
          <strong>❌ Server Unavailable</strong>
          <p style={{ margin: "8px 0" }}>
            The backend server is not responding. Please ensure the server is running and try again.
          </p>
          <button 
            onClick={retryConnection}
            disabled={isCheckingServer}
            style={{
              padding: "8px 16px",
              background: "#721c24",
              color: "white",
              border: "none",
              borderRadius: "4px",
              cursor: isCheckingServer ? "not-allowed" : "pointer",
              opacity: isCheckingServer ? 0.6 : 1
            }}
          >
            {isCheckingServer ? "Checking..." : "Retry Connection"}
          </button>
        </div>
      )}

      <div style={{ marginBottom: 20 }}>
        <label>Option:</label>
        <select
          value={selectedOptionId}
          onChange={(e) => setSelectedOptionId(e.target.value)}
          disabled={!serverAvailable}
          style={{ opacity: serverAvailable ? 1 : 0.5 }}
        >
          {options.length === 0 && (
            <option value="">No options available</option>
          )}
          {options.map((o) => (
            <option key={o.id} value={o.id}>
              {o.title}
            </option>
          ))}
        </select>
      </div>

      <div>
        <textarea
          rows={6}
          style={{ width: "100%" }}
          placeholder="Enter text..."
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </div>

      <div style={{ marginTop: 10 }}>
        <button 
          disabled={!canSubmit} 
          onClick={onSubmit}
          title={!serverAvailable ? "Server is unavailable" : ""}
        >
          {isSubmitting ? "Submitting..." : "Generate"}
        </button>

        <button onClick={reset} style={{ marginLeft: 10 }}>
          Reset
        </button>
        
        {!serverAvailable && (
          <span style={{ marginLeft: 10, color: "#721c24", fontSize: "14px" }}>
            ❌ Server unavailable - Generate button disabled
          </span>
        )}
      </div>

      <hr />

      <div>
        <p><b>Status:</b> {status || "idle"}</p>
        <p><b>Request ID:</b> {requestId || "—"}</p>

        {error && (
          <p style={{ 
            color: "#721c24",
            background: "#f8d7da",
            padding: "10px",
            borderRadius: "4px",
            border: "1px solid #f5c6cb"
          }}>
            {error}
          </p>
        )}

        {meta?.latencyMs && (
          <p>Latency: {meta.latencyMs} ms</p>
        )}

        {meta?.usage && (
          <p>
            Tokens: {meta.usage.inputTokens} / {meta.usage.outputTokens}
          </p>
        )}

        <pre style={{ background: "#eee", padding: 10 }}>
          {output || "Waiting..."}
        </pre>
      </div>
    </div>
  );
}
