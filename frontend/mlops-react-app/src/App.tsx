import { useEffect, useMemo, useRef, useState } from "react";
import "./App.css";

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

const API_BASE = (process.env.REACT_APP_API_BASE as string) || "/api";

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
  const [serverHealthInfo, setServerHealthInfo] = useState<any>({});
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

  // derived server status for display: running | idle | offline
  const serverDisplayStatus = !serverAvailable ? 'offline' : (status === 'running' ? 'running' : 'idle');

  // Poll backend health endpoint every 30s to update server availability
  useEffect(() => {
    let mounted = true;
    const backendRoot = API_BASE.replace(/\/api\/?$/, "") || "";
    const healthUrl = `${backendRoot}/health`;

    async function checkHealth() {
      try {
        const res = await fetch(healthUrl, {cache: 'no-store'});
        if (!mounted) return;
        if (res.ok) {
          setServerAvailable(true);
          try {
            const body = await res.json();
            setServerHealthInfo(body);
          } catch (_e) {
            setServerHealthInfo({});
          }
        } else {
          setServerAvailable(false);
          setServerHealthInfo({});
        }
      } catch (_err) {
        if (!mounted) return;
        setServerAvailable(false);
        setServerHealthInfo({});
      }
    }

    checkHealth();
    const iv = setInterval(checkHealth, 30000);
    return () => {
      mounted = false;
      clearInterval(iv);
    };
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
    <div className="app-shell">
      <div className="topbar">
        <div className="brand">
          <h1>LLM Prompt Runner</h1>
          <p className="small">Lightweight prompt playground</p>
        </div>
        <div className="controls">
          {/* Server status shown in server card only; topbar left intentionally minimal */}
        </div>
      </div>

      {!serverAvailable && (
        <div className="card" style={{marginTop:12}}>
          <strong style={{color:'#ffb4b4'}}>Server Unavailable</strong>
          <div className="small" style={{marginTop:8}}>The backend server is not responding.</div>
          <div style={{marginTop:12}}>
            <button className="btn" onClick={retryConnection} disabled={isCheckingServer}>{isCheckingServer? 'Checking...':'Retry'}</button>
          </div>
        </div>
      )}

      <div className="grid">
        <div>
          <div className="card">
            <label className="label">Option</label>
            <select className="select" value={selectedOptionId} onChange={(e)=>setSelectedOptionId(e.target.value)}>
              {options.map(o=> <option key={o.id} value={o.id}>{o.title}</option>)}
            </select>

            <label className="label" style={{marginTop:12}}>Input</label>
            <textarea className="textarea" placeholder="Enter text..." value={text} onChange={(e)=>setText(e.target.value)} />

            <div style={{display:'flex',justifyContent:'flex-start',alignItems:'center',marginTop:12}}>
              <div style={{display:'flex',gap:8}}>
                <button className="btn" onClick={onSubmit} disabled={!canSubmit}>{isSubmitting? 'Submitting...':'Generate'}</button>
                <button className="btn-ghost" onClick={reset}>Reset</button>
              </div>
            </div>

            <div className="meta">
              <div>Latency: <b>{meta?.latencyMs??'—'}</b> ms</div>
              <div>Tokens: <b>{meta?.usage?.inputTokens??'—'}</b> / <b>{meta?.usage?.outputTokens??'—'}</b></div>
            </div>
          </div>
        </div>

        <div>
          <div className="card status-card">
            <div style={{display:'flex',justifyContent:'space-between',gap:12}}>
              <div style={{flex:1}}>
                <div className="small">Request Status</div>
                <div style={{display:'flex',alignItems:'center',gap:10,marginTop:6}}>
                  <div style={{fontWeight:700}}>{serverDisplayStatus}</div>
                  {/* spinner when server is processing a request, idle/offline icons otherwise */}
                  {serverDisplayStatus === 'running' ? (
                    <div className="spinner" aria-hidden></div>
                  ) : (
                    <div className={`health-dot ${serverDisplayStatus === 'idle' ? 'idle' : 'unhealthy'}`}></div>
                  )}
                </div>

                {error && <div style={{marginTop:12,color:'#ff6b6b'}}>{error}</div>}
              </div>

              <div style={{minWidth:220}}>
                <div className="small">Server Health</div>
                <div className="server-info" style={{display:'flex',alignItems:'center',gap:10,marginTop:6}}>
                  <div className={`health-dot ${serverAvailable ? 'healthy' : 'unhealthy'}`} aria-hidden></div>
                    <div style={{fontWeight:700}}>{serverAvailable ? 'online' : 'offline'}</div>
                </div>
                  {serverHealthInfo && serverHealthInfo.uptime && (
                    <div className="small" style={{marginTop:8}}>uptime: {serverHealthInfo.uptime}</div>
                  )}
              </div>
            </div>
          </div>

          <div className="card output-card" style={{marginTop:16}}>
            <div>
              <div className="small" style={{marginBottom:8}}>Output</div>
            </div>

            <div className="output" style={{marginTop:8}}>{output || 'Waiting for generation...'}</div>

            <div style={{marginTop:12}}>
              <label className="label">Request ID</label>
              <input className="input" value={requestId || ''} disabled />
            </div>

            <div className="footer-note" style={{marginTop:12}}>Tip: switch options to try different prompt templates.</div>
          </div>
        </div>
      </div>
    </div>
  );
}
