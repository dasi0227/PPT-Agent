package service

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"sync"
)

// The in-memory implementation supports isolated service composition in tests.
type memoryResourceStore struct {
	mu   sync.Mutex
	rows map[string]model.Resource
}

func newMemoryResourceStore() *memoryResourceStore {
	return &memoryResourceStore{rows: map[string]model.Resource{}}
}

func resourceKey(kind, id string) string { return kind + "/" + id }

func cloneResource(r model.Resource) model.Resource { r.Tags = append([]string{}, r.Tags...); return r }

func (s *memoryResourceStore) check(r model.Resource) error {
	for _, v := range s.rows {
		if r.Type == "snippet" && v.Type == r.Type && v.ID != r.ID && v.NormalizedName == r.NormalizedName {
			return store.ErrResourceConflict
		}
	}
	for _, tag := range r.Tags {
		valid := false
		switch r.Type {
		case "theme":
			valid = model.ThemeTag(tag).Valid()
		case "component":
			valid = model.ComponentTag(tag).Valid()
		case "skill":
			valid = model.SkillTag(tag).Valid()
		case "snippet":
			switch tag {
			case "identity", "deliverable", "constraint", "git", "review", "other":
				valid = true
			}
		}
		if !valid {
			return store.ErrTagNotFound
		}
	}
	return nil
}

func (s *memoryResourceStore) CreateResource(_ context.Context, r model.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := resourceKey(r.Type, r.ID)
	if _, ok := s.rows[key]; ok {
		return store.ErrResourceConflict
	}
	if err := s.check(r); err != nil {
		return err
	}
	s.rows[key] = cloneResource(r)
	return nil
}

func (s *memoryResourceStore) GetResource(_ context.Context, kind, id string) (model.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[resourceKey(kind, id)]
	if !ok {
		return r, store.ErrResourceNotFound
	}
	return cloneResource(r), nil
}

func (s *memoryResourceStore) ListResources(_ context.Context, kind string) ([]model.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []model.Resource{}
	for _, r := range s.rows {
		if r.Type == kind {
			out = append(out, cloneResource(r))
		}
	}
	return out, nil
}

func (s *memoryResourceStore) UpdateResource(_ context.Context, r model.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := resourceKey(r.Type, r.ID)
	if _, ok := s.rows[key]; !ok {
		return store.ErrResourceNotFound
	}
	if err := s.check(r); err != nil {
		return err
	}
	s.rows[key] = cloneResource(r)
	return nil
}

func (s *memoryResourceStore) DeleteResource(_ context.Context, kind, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := resourceKey(kind, id)
	if _, ok := s.rows[key]; !ok {
		return store.ErrResourceNotFound
	}
	delete(s.rows, key)
	return nil
}
