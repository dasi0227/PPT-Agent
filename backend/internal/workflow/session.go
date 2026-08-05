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
	ErrInvalidArtifactPath  = errors.New("invalid artifact path")
	ErrArtifactHashMismatch = errors.New("artifact hash mismatch")
)

// sessionArtifact records one artifact touched during a run. Content is written
// straight to the project directory (file is the single source of truth); the
// baseline bytes captured on first touch let the completion gate compare the
// run's net change without a private write sandbox.
type sessionArtifact struct {
	Ref           ArtifactRef
	Source        string
	Relative      string
	BeforeContent []byte
	BeforeHash    string
	AfterHash     string
	Existed       bool
	Delete        bool
}

type WriteItem struct {
	Ref     ArtifactRef
	Source  string
	Content []byte
}

// RunSession is the run's direct-write handle. Typed tools write artifacts
// directly onto disk and the session tracks what changed so the completion gate
// and finalize step can observe the run. A failed or canceled run therefore
// leaves its partial products on disk instead of discarding them.
type RunSession struct {
	projectDir            string
	runID                 string
	artifacts             map[string]sessionArtifact
	materializationProofs []MaterializationProof
	closed                bool
}

// NewRunSession opens a direct-write session rooted at the project directory.
func NewRunSession(projectDir, runID string) (*RunSession, error) {
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	return &RunSession{
		projectDir: projectDir, runID: runID,
		artifacts: map[string]sessionArtifact{},
	}, nil
}

func (s *RunSession) HasChange(ref ArtifactRef) bool {
	relative, err := s.resolveRelative(ref)
	if err != nil {
		return false
	}
	entry, ok := s.artifacts[relative]
	return ok && !entry.Delete
}

// Write writes content directly to the project directory and records the change.
// Repeated writes of the same artifact in one run keep the run's baseline (the
// bytes present before the run touched it) and only advance the after hash.
func (s *RunSession) Write(ref ArtifactRef, source string, content []byte) (ArtifactChange, error) {
	relative, err := s.resolveRelative(ref)
	if err != nil {
		return ArtifactChange{}, err
	}
	if s.closed {
		return ArtifactChange{}, errors.New("run session is closed")
	}
	finalPath := filepath.Join(s.projectDir, relative)
	if previous, ok := s.artifacts[relative]; ok {
		if err := atomicWrite(finalPath, content); err != nil {
			return ArtifactChange{}, err
		}
		previous.Source, previous.AfterHash, previous.Delete = source, hashBytes(content), false
		s.artifacts[relative] = previous
		return ArtifactChange{Artifact: ref, BeforeHash: previous.BeforeHash, AfterHash: previous.AfterHash, Source: source}, nil
	}
	before, readErr := os.ReadFile(finalPath)
	existed := readErr == nil
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return ArtifactChange{}, readErr
	}
	if err := atomicWrite(finalPath, content); err != nil {
		return ArtifactChange{}, err
	}
	entry := sessionArtifact{
		Ref: ref, Source: source, Relative: relative,
		BeforeContent: append([]byte(nil), before...), BeforeHash: hashBytes(before),
		AfterHash: hashBytes(content), Existed: existed,
	}
	s.artifacts[relative] = entry
	return ArtifactChange{Artifact: ref, BeforeHash: entry.BeforeHash, AfterHash: entry.AfterHash, Source: source}, nil
}

// WriteBatch applies a logical domain-target update by writing every item to
// disk in order and recording each change.
func (s *RunSession) WriteBatch(items []WriteItem) ([]ArtifactChange, error) {
	if len(items) == 0 {
		return nil, errors.New("write batch is empty")
	}
	seen := map[string]bool{}
	for _, item := range items {
		relative, err := s.resolveRelative(item.Ref)
		if err != nil {
			return nil, err
		}
		if seen[relative] {
			return nil, fmt.Errorf("duplicate written artifact %s", relative)
		}
		seen[relative] = true
	}
	changes := make([]ArtifactChange, 0, len(items))
	for _, item := range items {
		change, err := s.Write(item.Ref, item.Source, item.Content)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func (s *RunSession) Read(ref ArtifactRef) ([]byte, error) {
	relative, err := s.resolveRelative(ref)
	if err != nil {
		return nil, err
	}
	if entry, ok := s.artifacts[relative]; ok && entry.Delete {
		return nil, fs.ErrNotExist
	}
	return os.ReadFile(filepath.Join(s.projectDir, relative))
}

// ReadBaseline returns the bytes that existed before this run first touched the
// artifact. It falls back to the on-disk content for untouched artifacts.
func (s *RunSession) ReadBaseline(ref ArtifactRef) ([]byte, error) {
	relative, err := s.resolveRelative(ref)
	if err != nil {
		return nil, err
	}
	if entry, ok := s.artifacts[relative]; ok {
		if !entry.Existed {
			return nil, fs.ErrNotExist
		}
		return append([]byte(nil), entry.BeforeContent...), nil
	}
	return os.ReadFile(filepath.Join(s.projectDir, relative))
}

func (s *RunSession) ProjectDir() string { return s.projectDir }

func (s *RunSession) ChangeSet() ChangeSet {
	out := EmptyChangeSet()
	keys := make([]string, 0, len(s.artifacts))
	for key := range s.artifacts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry := s.artifacts[key]
		if entry.Ref.Kind == ArtifactDerived {
			continue
		}
		change := ArtifactChange{
			Artifact: entry.Ref, BeforeHash: entry.BeforeHash,
			AfterHash: entry.AfterHash, Source: entry.Source,
			Tentative: strings.HasPrefix(entry.Source, "tentative:"),
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

// ValidateBaselines is retained for the completion gate. Direct writes own the
// project files and runs are serialized per project (RUN_ACTIVE), so there is
// no separate private baseline to reconcile.
func (s *RunSession) ValidateBaselines() error { return nil }

func (s *RunSession) MarkTentative() {
	for key, entry := range s.artifacts {
		entry.Source = "tentative:" + strings.TrimPrefix(entry.Source, "tentative:")
		s.artifacts[key] = entry
	}
}

func (s *RunSession) AcceptMaterializationProofs(proofs []MaterializationProof) {
	s.materializationProofs = append([]MaterializationProof(nil), proofs...)
}

// Commit finalizes the run. Artifacts are already on disk, so it only runs the
// finalize callback (version snapshots + run-time database metadata). A failure
// here does not roll back the already-written files.
func (s *RunSession) Commit(ctx context.Context, metadata CommitMetadata) error {
	if s.closed {
		return errors.New("run session is closed")
	}
	if metadata != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
		commitContext := CommitContext{
			Changes:               s.ChangeSet(),
			MaterializationProofs: append([]MaterializationProof(nil), s.materializationProofs...),
		}
		if err := metadata(ctx, commitContext); err != nil {
			return err
		}
	}
	s.closed = true
	return nil
}

func (s *RunSession) resolveRelative(ref ArtifactRef) (string, error) {
	relative := filepath.Clean(filepath.FromSlash(ref.Path))
	if relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrInvalidArtifactPath
	}
	full := filepath.Join(s.projectDir, relative)
	rel, err := filepath.Rel(s.projectDir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidArtifactPath
	}
	return filepath.ToSlash(rel), nil
}

func atomicWrite(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".runtime-*")
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
