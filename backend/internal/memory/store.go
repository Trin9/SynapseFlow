package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xunchenzheng/synapse/internal/store"
	"github.com/xunchenzheng/synapse/pkg/models"
)

// ExperienceStore persists and retrieves troubleshooting memories.
type ExperienceStore interface {
	Save(context.Context, *models.Experience) error
	List(context.Context) ([]models.Experience, error)
	Search(context.Context, store.SearchQuery) ([]models.Experience, error)
}

// InMemoryStore is the minimal Sprint 5 storage backend.
type InMemoryStore struct {
	mu          sync.RWMutex
	experiences map[string]models.Experience
	order       []string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		experiences: make(map[string]models.Experience),
		order:       make([]string, 0),
	}
}

func (s *InMemoryStore) Save(_ context.Context, exp *models.Experience) error {
	if exp == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.experiences[exp.ID]; !exists {
		s.order = append(s.order, exp.ID)
	}
	s.experiences[exp.ID] = *exp
	return nil
}

func (s *InMemoryStore) List(_ context.Context) ([]models.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.Experience, 0, len(s.order))
	for i := len(s.order) - 1; i >= 0; i-- {
		id := s.order[i]
		out = append(out, s.experiences[id])
	}
	return out, nil
}

func (s *InMemoryStore) Search(_ context.Context, query store.SearchQuery) ([]models.Experience, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topK := query.TopK
	if topK <= 0 {
		topK = 3
	}

	textTokens := tokenize(query.Text)
	type scored struct {
		exp   models.Experience
		score float64
	}
	scoredResults := make([]scored, 0, len(s.experiences))

	for _, id := range s.order {
		exp := s.experiences[id]
		if query.AlertType != "" && !strings.EqualFold(exp.AlertType, query.AlertType) {
			continue
		}
		if query.ServiceName != "" && !strings.EqualFold(exp.ServiceName, query.ServiceName) {
			continue
		}

		docTokens := tokenize(exp.Document + "\n" + exp.Summary + "\n" + exp.RootCause + "\n" + exp.ActionTaken)
		score := overlapScore(textTokens, docTokens)
		if score <= 0 {
			continue
		}
		exp.Score = score
		scoredResults = append(scoredResults, scored{exp: exp, score: score})
	}

	sort.SliceStable(scoredResults, func(i, j int) bool {
		if scoredResults[i].score == scoredResults[j].score {
			return scoredResults[i].exp.CreatedAt.After(scoredResults[j].exp.CreatedAt)
		}
		return scoredResults[i].score > scoredResults[j].score
	})

	if len(scoredResults) > topK {
		scoredResults = scoredResults[:topK]
	}

	out := make([]models.Experience, 0, len(scoredResults))
	for _, item := range scoredResults {
		out = append(out, item.exp)
	}
	return out, nil
}

func tokenize(text string) map[string]struct{} {
	parts := strings.Fields(strings.ToLower(strings.NewReplacer(",", " ", ".", " ", ":", " ", ";", " ", "\n", " ", "\t", " ").Replace(text)))
	set := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func overlapScore(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	matched := 0
	for token := range a {
		if _, ok := b[token]; ok {
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	denominator := len(a)
	if len(b) < denominator {
		denominator = len(b)
	}
	return float64(matched) / float64(denominator)
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
