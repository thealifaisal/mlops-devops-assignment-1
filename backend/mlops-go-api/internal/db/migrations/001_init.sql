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

-- Seed a default prompt used by the frontend and tests. Insert only if missing.
INSERT INTO prompts (id, title, description, template, model, temperature, max_tokens)
SELECT 'summarize_v1', 'Summarize Text', 'Summarize a block of text into a short paragraph.',
  'Please provide a concise summary for the following text:\n\n{{text}}', 'gpt-5-mini', 0.2, 512
WHERE NOT EXISTS (SELECT 1 FROM prompts WHERE id = 'summarize_v1');
