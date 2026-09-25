package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

type ResourceStore interface {
	CreateResource(context.Context, model.Resource) error
	GetResource(context.Context, string, string) (model.Resource, error)
	ListResources(context.Context, string) ([]model.Resource, error)
	UpdateResource(context.Context, model.Resource) error
	DeleteResource(context.Context, string, string) error
}

var repositoryWriteMu sync.Mutex

type ResourceService struct {
	root  string
	store ResourceStore
}

func NewResourceService(root WorkRoot, st ResourceStore) *ResourceService {
	return &ResourceService{root: string(root), store: st}
}

func resourceFile(kind string) (string, string, int64, error) {
	switch kind {
	case "theme":
		return "themes", "theme.css", maxRepositoryFileSize, nil
	case "component":
		return "components", "index.html", maxRepositoryFileSize, nil
	case "skill":
		return "skills", "SKILL.md", maxSkillFileBytes, nil
	case "snippet":
		return "snippets", "snippet.txt", 16 << 10, nil
	default:
		return "", "", 0, ErrInvalidRepositoryID
	}
}

func (s *ResourceService) directoryPath(kind, id string) (string, error) {
	folder, _, _, err := resourceFile(kind)
	if err != nil || !validRepositoryID(id) {
		return "", ErrInvalidRepositoryID
	}
	parent := s.root
	for _, part := range []string{"assets", folder, id} {
		parent = filepath.Join(parent, part)
		info, err := os.Lstat(parent)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrUnsafeRepositoryPath
		}
	}
	return parent, nil
}

func (s *ResourceService) payloadPath(kind, id string) (string, error) {
	dir, err := s.directoryPath(kind, id)
	if err != nil {
		return "", err
	}
	_, file, _, _ := resourceFile(kind)
	path := filepath.Join(dir, file)
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrUnsafeRepositoryPath
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return path, nil
}

func validateResourceContent(kind string, raw []byte) error {
	_, _, limit, err := resourceFile(kind)
	if err != nil {
		return err
	}
	if int64(len(raw)) > limit {
		return ErrRepositoryFileTooLarge
	}
	if !utf8.Valid(raw) || strings.TrimSpace(string(raw)) == "" {
		return fmt.Errorf("%w: 正文不能为空且必须为 UTF-8", ErrRepositoryCorrupt)
	}
	if kind == "theme" {
		return validateThemeTokens(raw)
	}
	return nil
}

func (s *ResourceService) readContent(kind, id string) ([]byte, string, error) {
	path, err := s.payloadPath(kind, id)
	if err != nil {
		return nil, "", err
	}
	folder, file, limit, _ := resourceFile(kind)
	raw, _, err := readRepositoryFile(filepath.Join(s.root, "assets", folder), id, file, limit)
	if err == nil {
		err = validateResourceContent(kind, raw)
	}
	return raw, path, err
}

func (s *ResourceService) Inspect(ctx context.Context, kind, id string) (model.Resource, []byte, string, model.ResourceContentState, error) {
	if err := validateResourceIdentity(kind, id); err != nil {
		return model.Resource{}, nil, "", model.ResourceContentState{}, err
	}
	r, err := s.store.GetResource(ctx, kind, id)
	if err != nil {
		return r, nil, "", model.ResourceContentState{}, err
	}
	raw, path, readErr := s.readContent(kind, id)
	state := model.ResourceContentState{ContentState: "ready"}
	if readErr != nil {
		state.ContentState = "invalid"
		state.ContentError = "资源文件无效：" + readErr.Error()
		raw = nil
		if errors.Is(readErr, fs.ErrNotExist) {
			state.ContentState = "missing"
			state.ContentError = "资源文件缺失"
		}
	}
	return r, raw, path, state, nil
}

func (s *ResourceService) Content(ctx context.Context, kind, id string) ([]byte, error) {
	if err := validateResourceIdentity(kind, id); err != nil {
		return nil, err
	}
	if _, err := s.store.GetResource(ctx, kind, id); err != nil {
		return nil, err
	}
	raw, _, err := s.readContent(kind, id)
	return raw, err
}

func validateResourceIdentity(kind, id string) error {
	if _, _, _, err := resourceFile(kind); err != nil || !validRepositoryID(id) {
		return ErrInvalidRepositoryID
	}
	return nil
}

func validateResource(r *model.Resource) error {
	if _, _, _, err := resourceFile(r.Type); err != nil || !validRepositoryID(r.ID) {
		return ErrInvalidRepositoryID
	}
	name, description, err := validateRepositoryMetadata(r.Name, r.Description)
	if err != nil {
		return err
	}
	if strings.ContainsAny(name, "\r\n\t") {
		return fmt.Errorf("%w: 名称不能包含换行或制表符", ErrRepositoryCorrupt)
	}
	r.Name = name
	r.Description = description
	r.NormalizedName = strings.ToLower(name)
	if r.Tags == nil {
		r.Tags = []string{}
	}
	if len(r.Tags) > 2 {
		return fmt.Errorf("%w: 最多选择两个标签", ErrRepositoryCorrupt)
	}
	seen := map[string]bool{}
	for _, tag := range r.Tags {
		if tag == "" || seen[tag] {
			return fmt.Errorf("%w: 标签不能为空或重复", ErrRepositoryCorrupt)
		}
		seen[tag] = true
	}
	return nil
}

func (s *ResourceService) Register(ctx context.Context, r model.Resource) (model.Resource, error) {
	repositoryWriteMu.Lock()
	defer repositoryWriteMu.Unlock()
	if err := validateResource(&r); err != nil {
		return r, err
	}
	if _, err := s.store.GetResource(ctx, r.Type, r.ID); err == nil {
		return r, store.ErrResourceConflict
	} else if !errors.Is(err, store.ErrResourceNotFound) {
		return r, err
	}
	if err := s.recoverOne(ctx, r.Type, r.ID); err != nil {
		return r, err
	}
	if _, _, err := s.readContent(r.Type, r.ID); err != nil {
		return r, err
	}
	r.CreatedAt = time.Now().Unix()
	r.UpdatedAt = r.CreatedAt
	if err := s.store.CreateResource(ctx, r); err != nil {
		return r, err
	}
	return r, nil
}

type ResourcePatch struct {
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Tags        *[]string `json:"tags"`
	Disabled    *bool     `json:"disabled"`
}

func (s *ResourceService) Patch(ctx context.Context, kind, id string, p ResourcePatch) (model.Resource, error) {
	if err := validateResourceIdentity(kind, id); err != nil {
		return model.Resource{}, err
	}
	repositoryWriteMu.Lock()
	defer repositoryWriteMu.Unlock()
	r, err := s.store.GetResource(ctx, kind, id)
	if err != nil {
		return r, err
	}
	if p.Name == nil && p.Description == nil && p.Tags == nil && p.Disabled == nil {
		return r, fmt.Errorf("%w: 至少提供一个修改字段", ErrRepositoryCorrupt)
	}
	if p.Name != nil {
		r.Name = *p.Name
	}
	if p.Description != nil {
		r.Description = *p.Description
	}
	if p.Tags != nil {
		r.Tags = *p.Tags
	}
	if p.Disabled != nil {
		r.Disabled = *p.Disabled
	}
	if err = validateResource(&r); err != nil {
		return r, err
	}
	r.UpdatedAt = time.Now().Unix()
	return r, s.store.UpdateResource(ctx, r)
}

// Trash paths are derived identities, never paths supplied by an API client.
func (s *ResourceService) trashDir(kind, id string) (string, error) {
	if _, _, _, err := resourceFile(kind); err != nil || !validRepositoryID(id) {
		return "", ErrInvalidRepositoryID
	}
	path := s.root
	for _, part := range []string{".resource-trash", kind, id} {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrUnsafeRepositoryPath
		}
	}
	return path, nil
}

func (s *ResourceService) recoverOne(ctx context.Context, kind, id string) error {
	trash, err := s.trashDir(kind, id)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(trash); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	_, err = s.store.GetResource(ctx, kind, id)
	if errors.Is(err, store.ErrResourceNotFound) {
		return os.RemoveAll(trash)
	}
	if err != nil {
		return err
	}
	dir, err := s.directoryPath(kind, id)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("资源 %s/%s 删除恢复目标已存在", kind, id)
	}
	if err = os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}
	return os.Rename(trash, dir)
}

func (s *ResourceService) RecoverDeletes(ctx context.Context) error {
	repositoryWriteMu.Lock()
	defer repositoryWriteMu.Unlock()
	for _, kind := range []string{"theme", "component", "skill", "snippet"} {
		probe, err := s.trashDir(kind, "probe")
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(filepath.Dir(probe))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = s.recoverOne(ctx, kind, entry.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ResourceService) Delete(ctx context.Context, kind, id string) error {
	repositoryWriteMu.Lock()
	defer repositoryWriteMu.Unlock()
	if err := s.recoverOne(ctx, kind, id); err != nil {
		return err
	}
	if _, err := s.store.GetResource(ctx, kind, id); err != nil {
		return err
	}
	dir, err := s.directoryPath(kind, id)
	if err != nil {
		return err
	}
	trash, err := s.trashDir(kind, id)
	if err != nil {
		return err
	}
	moved := false
	if _, err = os.Lstat(dir); err == nil {
		if err = os.MkdirAll(filepath.Dir(trash), 0755); err != nil {
			return err
		}
		if err = os.Rename(dir, trash); err != nil {
			return err
		}
		moved = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err = s.store.DeleteResource(ctx, kind, id); err != nil {
		if moved {
			return errors.Join(err, os.Rename(trash, dir))
		}
		return err
	}
	if moved {
		return os.RemoveAll(trash)
	}
	return nil
}

func (s *ResourceService) CreateSnippet(ctx context.Context, r model.Resource, content string) (model.Snippet, error) {
	repositoryWriteMu.Lock()
	defer repositoryWriteMu.Unlock()
	r.Type = "snippet"
	r.ID = model.MustShortID("snp")
	if err := validateResource(&r); err != nil {
		return model.Snippet{}, err
	}
	raw := []byte(content)
	if err := validateResourceContent(r.Type, raw); err != nil {
		return model.Snippet{}, err
	}
	path, err := s.payloadPath(r.Type, r.ID)
	if err != nil {
		return model.Snippet{}, err
	}
	parent := filepath.Dir(filepath.Dir(path))
	if err = os.MkdirAll(parent, 0755); err != nil {
		return model.Snippet{}, err
	}
	staging, err := os.MkdirTemp(parent, ".snippet-")
	if err != nil {
		return model.Snippet{}, err
	}
	defer os.RemoveAll(staging)
	if err = atomicRewriteRepositoryFile(filepath.Join(staging, "snippet.txt"), raw); err != nil {
		return model.Snippet{}, err
	}
	dir := filepath.Dir(path)
	if _, err = os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		return model.Snippet{}, store.ErrResourceConflict
	}
	if err = os.Rename(staging, dir); err != nil {
		return model.Snippet{}, err
	}
	r.CreatedAt = time.Now().Unix()
	r.UpdatedAt = r.CreatedAt
	if err = s.store.CreateResource(ctx, r); err != nil {
		return model.Snippet{}, errors.Join(err, os.RemoveAll(dir))
	}
	return s.Snippet(ctx, r.ID)
}

func (s *ResourceService) Snippet(ctx context.Context, id string) (model.Snippet, error) {
	r, raw, path, state, err := s.Inspect(ctx, "snippet", id)
	return model.Snippet{Resource: r, ResourceContentState: state, Content: string(raw), OpenURL: repositoryOpenURL(path)}, err
}

func (s *ResourceService) Snippets(ctx context.Context) ([]model.Snippet, error) {
	rows, err := s.store.ListResources(ctx, "snippet")
	if err != nil {
		return nil, err
	}
	out := []model.Snippet{}
	for _, r := range rows {
		v, err := s.Snippet(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *ResourceService) WriteSnippet(ctx context.Context, id, content string) (model.Snippet, error) {
	if err := validateResourceIdentity("snippet", id); err != nil {
		return model.Snippet{}, err
	}
	repositoryWriteMu.Lock()
	defer repositoryWriteMu.Unlock()
	r, err := s.store.GetResource(ctx, "snippet", id)
	if err != nil {
		return model.Snippet{}, err
	}
	if err = validateResourceContent("snippet", []byte(content)); err != nil {
		return model.Snippet{}, err
	}
	path, err := s.payloadPath("snippet", id)
	if err != nil {
		return model.Snippet{}, err
	}
	original, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return model.Snippet{}, readErr
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return model.Snippet{}, err
	}
	if err = atomicRewriteRepositoryFile(path, []byte(content)); err != nil {
		return model.Snippet{}, err
	}
	r.UpdatedAt = time.Now().Unix()
	if err = s.store.UpdateResource(ctx, r); err != nil {
		var rollback error
		if errors.Is(readErr, fs.ErrNotExist) {
			rollback = os.Remove(path)
		} else {
			rollback = atomicRewriteRepositoryFile(path, original)
		}
		return model.Snippet{}, errors.Join(err, rollback)
	}
	return s.Snippet(ctx, id)
}

// Tags reads the dictionary without exposing a tag editing surface.
func (s *ResourceService) Tags(ctx context.Context, scope string) ([]model.TagDefinition, error) {
	if scope != "" {
		if _, _, _, err := resourceFile(scope); err != nil {
			return nil, err
		}
	}
	dictionary, ok := s.store.(interface {
		ListTags(context.Context, string) ([]model.TagDefinition, error)
	})
	if !ok {
		return nil, errors.New("resource tag dictionary is unavailable")
	}
	return dictionary.ListTags(ctx, scope)
}
