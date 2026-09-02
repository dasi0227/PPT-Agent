package service

import (
	"context"
	"sync"
	"time"
)

const (
	resourceTypeTheme     = "theme"
	resourceTypeComponent = "component"
	resourceTypeSkill     = "skill"
	resourceTypePrompt    = "prompt"
)

type repositoryMetadataStore interface {
	ListResourceTagKeys(ctx context.Context, resourceType, resourceID string) ([]string, error)
	ReplaceResourceTagKeys(ctx context.Context, resourceType, resourceID string, tagKeys []string) error
	GetResourceDisabled(ctx context.Context, resourceType, resourceID string) (bool, error)
	SetResourceDisabled(ctx context.Context, resourceType, resourceID string, disabled bool, updatedAt int64) error
	DeleteResourceMetadata(ctx context.Context, resourceType, resourceID string) error
}

type memoryRepositoryMetadataStore struct {
	mu       sync.Mutex
	tags     map[string][]string
	disabled map[string]bool
}

func newMemoryRepositoryMetadataStore() *memoryRepositoryMetadataStore {
	return &memoryRepositoryMetadataStore{
		tags:     map[string][]string{},
		disabled: map[string]bool{},
	}
}

func repositoryMetadataKey(resourceType, resourceID string) string {
	return resourceType + "\x00" + resourceID
}

func repositoryMetadataOrMemory(stores []repositoryMetadataStore) repositoryMetadataStore {
	if len(stores) > 0 && stores[0] != nil {
		return stores[0]
	}
	return newMemoryRepositoryMetadataStore()
}

func (s *memoryRepositoryMetadataStore) ListResourceTagKeys(_ context.Context, resourceType, resourceID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.tags[repositoryMetadataKey(resourceType, resourceID)]...), nil
}

func (s *memoryRepositoryMetadataStore) ReplaceResourceTagKeys(_ context.Context, resourceType, resourceID string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags[repositoryMetadataKey(resourceType, resourceID)] = append([]string(nil), tagKeys...)
	return nil
}

func (s *memoryRepositoryMetadataStore) GetResourceDisabled(_ context.Context, resourceType, resourceID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disabled[repositoryMetadataKey(resourceType, resourceID)], nil
}

func (s *memoryRepositoryMetadataStore) SetResourceDisabled(_ context.Context, resourceType, resourceID string, disabled bool, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disabled[repositoryMetadataKey(resourceType, resourceID)] = disabled
	return nil
}

func (s *memoryRepositoryMetadataStore) DeleteResourceMetadata(_ context.Context, resourceType, resourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := repositoryMetadataKey(resourceType, resourceID)
	delete(s.tags, key)
	delete(s.disabled, key)
	return nil
}

func repositoryStateTimestamp() int64 {
	return time.Now().Unix()
}
