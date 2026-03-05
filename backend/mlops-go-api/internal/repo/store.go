package repo

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"mlops-go-api/internal/model"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	mu      sync.RWMutex
	prompts map[string]*model.Prompt
	reqs    map[string]*model.Request
}

func NewStore() *Store {
	s := &Store{
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

func (s *Store) ListPrompts() []*model.Prompt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Prompt, 0, len(s.prompts))
	for _, p := range s.prompts {
		out = append(out, p)
	}
	return out
}

func (s *Store) GetPrompt(id string) (*model.Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.prompts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (s *Store) CreateRequest(promptID string, input map[string]interface{}) (*model.Request, error) {
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

func (s *Store) GetRequest(id string) (*model.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.reqs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *Store) UpdateRequest(r *model.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.UpdatedAt = time.Now()
	s.reqs[r.ID] = r
	return nil
}
