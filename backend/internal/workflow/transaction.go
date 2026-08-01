package workflow

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrInvalidArtifactPath = errors.New("invalid artifact path")
	ErrStagedHashMismatch  = errors.New("staged artifact hash mismatch")
)

type stagedArtifact struct {
	Ref        ArtifactRef
	StepID     string
	Relative   string
	BeforeHash string
	AfterHash  string
	Existed    bool
	Delete     bool
}

type Transaction struct {
	projectDir string
	runID      string
	root       string
	artifacts  map[string]stagedArtifact
	closed     bool
}

func NewTransaction(projectDir, runID string) (*Transaction, error) {
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(projectDir, ".staging", runID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &Transaction{
		projectDir: projectDir, runID: runID, root: root,
		artifacts: map[string]stagedArtifact{},
	}, nil
}

func (t *Transaction) Root() string { return t.root }

func (t *Transaction) Stage(ref ArtifactRef, stepID string, content []byte) (ArtifactChange, error) {
	relative, err := t.resolveRelative(ref)
	if err != nil {
		return ArtifactChange{}, err
	}
	if t.closed {
		return ArtifactChange{}, errors.New("staging transaction is closed")
	}
	finalPath := filepath.Join(t.projectDir, relative)
	before, readErr := os.ReadFile(finalPath)
	existed := readErr == nil
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return ArtifactChange{}, readErr
	}
	stagedPath := filepath.Join(t.root, relative)
	if err := atomicWrite(stagedPath, content); err != nil {
		return ArtifactChange{}, err
	}
	entry := stagedArtifact{
		Ref: ref, StepID: stepID, Relative: relative, BeforeHash: hashBytes(before),
		AfterHash: hashBytes(content), Existed: existed,
	}
	t.artifacts[relative] = entry
	return ArtifactChange{Artifact: ref, BeforeHash: entry.BeforeHash, AfterHash: entry.AfterHash, StepID: stepID}, nil
}

func (t *Transaction) StageDelete(ref ArtifactRef, stepID string) (ArtifactChange, error) {
	relative, err := t.resolveRelative(ref)
	if err != nil {
		return ArtifactChange{}, err
	}
	before, readErr := os.ReadFile(filepath.Join(t.projectDir, relative))
	if readErr != nil {
		return ArtifactChange{}, readErr
	}
	entry := stagedArtifact{
		Ref: ref, StepID: stepID, Relative: relative, BeforeHash: hashBytes(before),
		AfterHash: hashBytes(nil), Existed: true, Delete: true,
	}
	t.artifacts[relative] = entry
	return ArtifactChange{Artifact: ref, BeforeHash: entry.BeforeHash, AfterHash: entry.AfterHash, StepID: stepID}, nil
}

func (t *Transaction) Read(ref ArtifactRef) ([]byte, error) {
	relative, err := t.resolveRelative(ref)
	if err != nil {
		return nil, err
	}
	if entry, ok := t.artifacts[relative]; ok {
		if entry.Delete {
			return nil, fs.ErrNotExist
		}
		return os.ReadFile(filepath.Join(t.root, relative))
	}
	return os.ReadFile(filepath.Join(t.projectDir, relative))
}

func (t *Transaction) StagedPath(ref ArtifactRef) (string, error) {
	relative, err := t.resolveRelative(ref)
	if err != nil {
		return "", err
	}
	if entry, ok := t.artifacts[relative]; ok && !entry.Delete {
		return filepath.Join(t.root, relative), nil
	}
	return filepath.Join(t.projectDir, relative), nil
}

func (t *Transaction) ChangeSet() ChangeSet {
	out := EmptyChangeSet()
	keys := make([]string, 0, len(t.artifacts))
	for key := range t.artifacts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry := t.artifacts[key]
		change := ArtifactChange{
			Artifact: entry.Ref, BeforeHash: entry.BeforeHash,
			AfterHash: entry.AfterHash, StepID: entry.StepID,
		}
		switch {
		case entry.Delete:
			out.Deleted = append(out.Deleted, change)
		case entry.Existed:
			out.Updated = append(out.Updated, change)
		default:
			out.Created = append(out.Created, change)
		}
	}
	return out
}

type CommitMetadata func(context.Context, ChangeSet) error

func (t *Transaction) Commit(ctx context.Context, metadata CommitMetadata) error {
	if t.closed {
		return errors.New("staging transaction is closed")
	}
	keys := make([]string, 0, len(t.artifacts))
	for key := range t.artifacts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	before := make(map[string][]byte, len(keys))
	existed := make(map[string]bool, len(keys))
	for _, key := range keys {
		entry := t.artifacts[key]
		finalPath := filepath.Join(t.projectDir, key)
		raw, err := os.ReadFile(finalPath)
		if err == nil {
			before[key], existed[key] = raw, true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if hashBytes(raw) != entry.BeforeHash {
			return fmt.Errorf("%w: %s source changed", ErrStagedHashMismatch, key)
		}
		if !entry.Delete {
			staged, err := os.ReadFile(filepath.Join(t.root, key))
			if err != nil {
				return err
			}
			if hashBytes(staged) != entry.AfterHash {
				return fmt.Errorf("%w: %s", ErrStagedHashMismatch, key)
			}
		}
	}
	restore := func() {
		for _, key := range keys {
			path := filepath.Join(t.projectDir, key)
			if existed[key] {
				_ = atomicWrite(path, before[key])
			} else {
				_ = os.Remove(path)
			}
		}
	}
	for _, key := range keys {
		entry := t.artifacts[key]
		finalPath := filepath.Join(t.projectDir, key)
		if entry.Delete {
			if err := os.Remove(finalPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				restore()
				return err
			}
			continue
		}
		staged, err := os.ReadFile(filepath.Join(t.root, key))
		if err != nil {
			restore()
			return err
		}
		if err := atomicWrite(finalPath, staged); err != nil {
			restore()
			return err
		}
	}
	if metadata != nil {
		if err := metadata(ctx, t.ChangeSet()); err != nil {
			restore()
			return err
		}
	}
	t.closed = true
	return t.Cleanup()
}

func (t *Transaction) Cleanup() error {
	t.closed = true
	err := os.RemoveAll(t.root)
	parent := filepath.Dir(t.root)
	_ = os.Remove(parent)
	return err
}

func (t *Transaction) resolveRelative(ref ArtifactRef) (string, error) {
	relative := filepath.Clean(filepath.FromSlash(ref.Path))
	if relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrInvalidArtifactPath
	}
	full := filepath.Join(t.projectDir, relative)
	rel, err := filepath.Rel(t.projectDir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidArtifactPath
	}
	return filepath.ToSlash(rel), nil
}

func atomicWrite(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pev-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(content); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum)
}
