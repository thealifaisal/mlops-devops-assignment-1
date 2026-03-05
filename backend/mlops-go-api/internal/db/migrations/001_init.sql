-- init schema for prompts and requests (v0)
CREATE TABLE IF NOT EXISTS prompts (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT,
  template TEXT NOT NULL,
  model TEXT NOT NULL,
  temperature DOUBLE PRECISION DEFAULT 0.2,
  max_tokens INT DEFAULT 512,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS requests (
  id UUID PRIMARY KEY,
  prompt_id TEXT REFERENCES prompts(id),
  input_json JSONB,
  status TEXT,
  result_text TEXT,
  error_message TEXT,
  usage_json JSONB,
  latency_ms INT,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP DEFAULT now(),
  started_at TIMESTAMP,
  finished_at TIMESTAMP
);
