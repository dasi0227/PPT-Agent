package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// InitializeResources is an explicit operation, never a server startup hook.
// Existing registered resources (including their user edits) are left untouched.
func (s *ResourceService) InitializeResources(ctx context.Context, seedRoot string) error {
	if err := s.initializeTags(ctx, seedRoot); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(seedRoot, "resources.json"))
	if err != nil {
		return err
	}
	var entries []model.Resource
	if err = json.Unmarshal(raw, &entries); err != nil {
		return err
	}
	for _, r := range entries {
		if _, err = s.store.GetResource(ctx, r.Type, r.ID); err == nil {
			continue
		} else if !errors.Is(err, store.ErrResourceNotFound) {
			return err
		}
		if err = validateResource(&r); err != nil {
			return err
		}
		source := NewResourceService(WorkRoot(seedRoot), s.store)
		body, _, err := source.readContent(r.Type, r.ID)
		if err != nil {
			return fmt.Errorf("seed %s/%s: %w", r.Type, r.ID, err)
		}
		path, err := s.payloadPath(r.Type, r.ID)
		if err != nil {
			return err
		}
		if _, err = os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			// Exclusive creation never overwrites an external file that appeared meanwhile.
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_, writeErr := file.Write(body)
			syncErr := file.Sync()
			closeErr := file.Close()
			if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
				_ = os.Remove(path)
				return err
			}
		} else if err != nil {
			return err
		}
		if _, err = s.Register(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// ReplacePresets is an explicit maintenance operation for the theme/component catalogue.
// Run it while the server is stopped; user enable/disable choices are retained.
func (s *ResourceService) ReplacePresets(ctx context.Context, seedRoot string, componentsOnly bool) error {
	if err := s.initializeTags(ctx, seedRoot); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(seedRoot, "resources.json"))
	if err != nil {
		return err
	}
	var entries []model.Resource
	if err = json.Unmarshal(raw, &entries); err != nil {
		return err
	}
	source := NewResourceService(WorkRoot(seedRoot), s.store)
	// Check every input and destination before writing any preset.
	for _, r := range entries {
		if r.Type != "component" && (componentsOnly || r.Type != "theme") {
			continue
		}
		if err = validateResource(&r); err != nil {
			return err
		}
		if _, _, err = source.readContent(r.Type, r.ID); err != nil {
			return err
		}
		if _, err = s.payloadPath(r.Type, r.ID); err != nil {
			return err
		}
	}
	for _, r := range entries {
		if r.Type != "component" && (componentsOnly || r.Type != "theme") {
			continue
		}
		body, _, err := source.readContent(r.Type, r.ID)
		if err != nil {
			return err
		}
		if err = s.replacePreset(ctx, r, body); err != nil {
			return err
		}
	}
	if !componentsOnly {
		for _, id := range []string{"swiss-modern", "corporate-clean", "warm-pastel", "tokyo-night", "xiaohongshu-white"} {
			if err = s.Delete(ctx, "theme", id); err != nil && !errors.Is(err, store.ErrResourceNotFound) {
				return err
			}
		}
	}
	return nil
}

func (s *ResourceService) replacePreset(ctx context.Context, r model.Resource, body []byte) error {
	repositoryWriteMu.Lock()
	defer repositoryWriteMu.Unlock()
	if err := validateResource(&r); err != nil {
		return err
	}
	previous, err := s.store.GetResource(ctx, r.Type, r.ID)
	exists := err == nil
	if err != nil && !errors.Is(err, store.ErrResourceNotFound) {
		return err
	}
	path, err := s.payloadPath(r.Type, r.ID)
	if err != nil {
		return err
	}
	original, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err = atomicRewriteRepositoryFile(path, body); err != nil {
		return err
	}
	r.CreatedAt = time.Now().Unix()
	r.UpdatedAt = r.CreatedAt
	if exists {
		r.CreatedAt = previous.CreatedAt
		r.Disabled = previous.Disabled
		err = s.store.UpdateResource(ctx, r)
	} else {
		err = s.store.CreateResource(ctx, r)
	}
	if err != nil {
		var rollback error
		if errors.Is(readErr, os.ErrNotExist) {
			rollback = os.Remove(path)
		} else {
			rollback = atomicRewriteRepositoryFile(path, original)
		}
		return errors.Join(err, rollback)
	}
	return nil
}

func (s *ResourceService) initializeTags(ctx context.Context, seedRoot string) error {
	raw, err := os.ReadFile(filepath.Join(seedRoot, "tags.json"))
	if err != nil {
		return err
	}
	var tags []model.TagDefinition
	if err := json.Unmarshal(raw, &tags); err != nil {
		return err
	}
	initializer, ok := s.store.(interface {
		InitializeTags(context.Context, []model.TagDefinition) error
	})
	if !ok {
		return errors.New("resource tag initializer is unavailable")
	}
	return initializer.InitializeTags(ctx, tags)
}
