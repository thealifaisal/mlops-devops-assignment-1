package repo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"mlops-go-api/internal/model"
)

var ErrNotFound = errors.New("not found")

// Repo defines the methods needed by the service layer.
type Repo interface {
	ListPrompts() ([]*model.Prompt, error)
	GetPrompt(id string) (*model.Prompt, error)
	CreateRequest(promptID string, input map[string]interface{}) (*model.Request, error)
	GetRequest(id string) (*model.Request, error)
	UpdateRequest(r *model.Request) error
}

// memStore is an in-memory implementation used for local dev or when DB isn't configured.
type memStore struct {
	mu      sync.RWMutex
	prompts map[string]*model.Prompt
	reqs    map[string]*model.Request
}

func NewStore() Repo {
	s := &memStore{
		prompts: make(map[string]*model.Prompt),
		reqs:    make(map[string]*model.Request),
	}

	// Seed with example prompt from README
	s.prompts["summarize_v1"] = &model.Prompt{
		ID:          "summarize_v1",
		Title:       "Summarize",
		Description: "Summarize text into bullet points",
		Template:    "Summarize the following text in 5 bullet points:\n\n{{text}}",
		Model:       "gpt-4o-mini",
	}

	return s
}

func (s *memStore) ListPrompts() ([]*model.Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Prompt, 0, len(s.prompts))
	for _, p := range s.prompts {
		out = append(out, p)
	}
	return out, nil
}

func (s *memStore) GetPrompt(id string) (*model.Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.prompts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (s *memStore) CreateRequest(promptID string, input map[string]interface{}) (*model.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := uuid.New().String()
	now := time.Now()
	r := &model.Request{
		ID:        id,
		PromptID:  promptID,
		InputJSON: input,
		Status:    "queued",
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.reqs[id] = r
	return r, nil
}

func (s *memStore) GetRequest(id string) (*model.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.reqs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *memStore) UpdateRequest(r *model.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.UpdatedAt = time.Now()
	s.reqs[r.ID] = r
	return nil
}

// dbStore is a Postgres-backed implementation.
type dbStore struct {
	db *sql.DB
}

func NewDBStore(db *sql.DB) Repo {
	return &dbStore{db: db}
}

func (d *dbStore) ListPrompts() ([]*model.Prompt, error) {
	rows, err := d.db.Query("SELECT id, title, description, template, model FROM prompts WHERE is_active = true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.Prompt{}
	for rows.Next() {
		var p model.Prompt
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.Template, &p.Model); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, nil
}

func (d *dbStore) GetPrompt(id string) (*model.Prompt, error) {
	var p model.Prompt
	err := d.db.QueryRow("SELECT id, title, description, template, model FROM prompts WHERE id=$1", id).Scan(&p.ID, &p.Title, &p.Description, &p.Template, &p.Model)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *dbStore) CreateRequest(promptID string, input map[string]interface{}) (*model.Request, error) {
	id := uuid.New().String()
	inputB, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	_, err = d.db.Exec("INSERT INTO requests (id, prompt_id, input_json, status, created_at, updated_at) VALUES ($1,$2,$3,$4,now(),now())", id, promptID, inputB, "queued")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &model.Request{ID: id, PromptID: promptID, InputJSON: input, Status: "queued", CreatedAt: now, UpdatedAt: now}, nil
}

func (d *dbStore) GetRequest(id string) (*model.Request, error) {
	var r model.Request
	var inputB, usageB sql.NullString
	var startedAt, finishedAt sql.NullTime
	err := d.db.QueryRow("SELECT id, prompt_id, input_json, status, result_text, error_message, usage_json, latency_ms, created_at, updated_at, started_at, finished_at FROM requests WHERE id=$1", id).
		Scan(&r.ID, &r.PromptID, &inputB, &r.Status, &r.ResultText, &r.Error, &usageB, &r.LatencyMS, &r.CreatedAt, &r.UpdatedAt, &startedAt, &finishedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if inputB.Valid {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(inputB.String), &m); err == nil {
			r.InputJSON = m
		}
	}
	if usageB.Valid {
		var um map[string]int
		if err := json.Unmarshal([]byte(usageB.String), &um); err == nil {
			r.Usage = um
		}
	}
	if startedAt.Valid {
		r.StartedAt = startedAt.Time
	}
	if finishedAt.Valid {
		r.FinishedAt = finishedAt.Time
	}
	return &r, nil
}

func (d *dbStore) UpdateRequest(r *model.Request) error {
	var usageB []byte
	var err error
	if r.Usage != nil {
		usageB, err = json.Marshal(r.Usage)
		if err != nil {
			return err
		}
	}
	_, err = d.db.Exec("UPDATE requests SET status=$1, result_text=$2, error_message=$3, usage_json=$4, latency_ms=$5, updated_at=now(), started_at=$6, finished_at=$7 WHERE id=$8",
		r.Status, r.ResultText, r.Error, usageB, r.LatencyMS, nullTime(r.StartedAt), nullTime(r.FinishedAt), r.ID)
	return err
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}
