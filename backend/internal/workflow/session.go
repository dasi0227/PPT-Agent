package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var activeRunSessions sync.Map

var (
	ErrInvalidArtifactPath  = errors.New("invalid artifact path")
	ErrArtifactHashMismatch = errors.New("artifact hash mismatch")
)

// sessionArtifact records one artifact in the current tool-call transaction.
// A successful tool call commits and clears these entries before the next call.
type sessionArtifact struct {
	Ref           ArtifactRef
	Source        string
	Relative      string
	BeforeContent []byte
	AfterContent  []byte
	BeforeHash    string
	AfterHash     string
	Existed       bool
	Delete        bool
}

// RunSession stages only the currently executing tool call. Completed calls are
// durably written to the project and are never discarded with the rest of a Run.
type RunSession struct {
	projectDir         string
	runID              string
	artifacts          map[string]sessionArtifact
	generationInputs   map[string]json.RawMessage
	closed             bool
	readBaselines      map[string]sessionBaseline
	requiredSlides     map[string]bool
	unchangedResources map[string]unchangedResource
}

type unchangedResource struct {
	resource Resource
	ref      ArtifactRef
	hash     string
}

type sessionBaseline struct {
	content []byte
	exists  bool
}

// Operation sessions are private to one worker and never become the active
// project overlay. Only the coordinator commits them or advances Run state.
func (s *RunSession) operationSession() *RunSession {
	return &RunSession{projectDir: s.projectDir, runID: s.runID,
		artifacts: map[string]sessionArtifact{}, readBaselines: map[string]sessionBaseline{},
		requiredSlides: map[string]bool{}}
}

func (s *RunSession) requireSlide(id string) {
	if s.requiredSlides == nil {
		s.requiredSlides = map[string]bool{}
	}
	s.requiredSlides[id] = true
}

func (s *RunSession) recordUnchangedResource(resource Resource, ref ArtifactRef, raw []byte) {
	if _, staged := s.artifacts[ref.Path]; staged {
		return
	}
	if s.unchangedResources == nil {
		s.unchangedResources = map[string]unchangedResource{}
	}
	s.unchangedResources[resource.Key()] = unchangedResource{resource: resource, ref: ref, hash: resourceContentHash(resource, raw)}
}

func (s *RunSession) readOriginal(relative string) ([]byte, error) {
	if baseline, ok := s.readBaselines[relative]; ok {
		if !baseline.exists {
			return nil, fs.ErrNotExist
		}
		return append([]byte(nil), baseline.content...), nil
	}
	release := pptmutation.ReadLockProject(s.projectDir)
	defer release()
	raw, err := os.ReadFile(filepath.Join(s.projectDir, relative))
	if s.readBaselines != nil && (err == nil || errors.Is(err, fs.ErrNotExist)) {
		s.readBaselines[relative] = sessionBaseline{content: append([]byte(nil), raw...), exists: err == nil}
	}
	return raw, err
}

type RunSessionSnapshot struct {
	RunID     string                    `json:"run_id"`
	Artifacts []RunSessionArtifactState `json:"artifacts"`
}

type RunSessionArtifactState struct {
	Ref           ArtifactRef `json:"ref"`
	Source        string      `json:"source"`
	Relative      string      `json:"relative"`
	BeforeContent []byte      `json:"before_content"`
	AfterContent  []byte      `json:"after_content"`
	BeforeHash    string      `json:"before_hash"`
	AfterHash     string      `json:"after_hash"`
	Existed       bool        `json:"existed"`
	Delete        bool        `json:"delete"`
}

type MutationJournal struct {
	RunID            string                     `json:"run_id"`
	OperationID      string                     `json:"operation_id"`
	RequestHash      string                     `json:"request_hash"`
	Committed        bool                       `json:"committed"`
	Artifacts        []RunSessionArtifactState  `json:"artifacts"`
	GenerationInputs map[string]json.RawMessage `json:"generation_inputs,omitempty"`
}

type MutationReceiptLookup func(context.Context, string, string) (string, bool, error)

// NewRunSession opens an isolated tool-call transaction rooted at the project directory.
func NewRunSession(projectDir, runID string) (*RunSession, error) {
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	if existing := ActiveRunSession(projectDir); existing != nil && existing.runID == runID && !existing.closed {
		return existing, nil
	}
	session := &RunSession{
		projectDir: projectDir, runID: runID,
		artifacts: map[string]sessionArtifact{},
	}
	activeRunSessions.Store(projectDir, session)
	return session, nil
}

func (s *RunSession) Snapshot() *RunSessionSnapshot {
	if s == nil || s.closed {
		return nil
	}
	keys := make([]string, 0, len(s.artifacts))
	for key := range s.artifacts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	snapshot := &RunSessionSnapshot{RunID: s.runID, Artifacts: make([]RunSessionArtifactState, 0, len(keys))}
	for _, key := range keys {
		entry := s.artifacts[key]
		snapshot.Artifacts = append(snapshot.Artifacts, RunSessionArtifactState{
			Ref: entry.Ref, Source: entry.Source, Relative: entry.Relative,
			BeforeContent: append([]byte(nil), entry.BeforeContent...),
			AfterContent:  append([]byte(nil), entry.AfterContent...),
			BeforeHash:    entry.BeforeHash, AfterHash: entry.AfterHash,
			Existed: entry.Existed, Delete: entry.Delete,
		})
	}
	return snapshot
}

func ActiveRunSession(projectDir string) *RunSession {
	abs, _ := filepath.Abs(projectDir)
	value, _ := activeRunSessions.Load(abs)
	session, _ := value.(*RunSession)
	return session
}
func (s *RunSession) ReadPath(path string) ([]byte, error) {
	return s.Read(ArtifactRef{Kind: ArtifactDerived, ID: path, Path: path})
}
func (s *RunSession) Discard() {
	s.resetOperation()
	activeRunSessions.CompareAndDelete(s.projectDir, s)
	s.closed = true
}

func (s *RunSession) resetOperation() {
	s.artifacts = map[string]sessionArtifact{}
	s.generationInputs = nil
	if s.readBaselines != nil {
		s.readBaselines = map[string]sessionBaseline{}
	}
	s.requiredSlides = nil
	s.unchangedResources = nil
}

// RollbackOperation abandons only the current tool call. Artifacts committed by
// earlier calls already live on disk and are intentionally left untouched.
func (s *RunSession) RollbackOperation() {
	if s == nil || s.closed {
		return
	}
	s.resetOperation()
}

func (s *RunSession) HasChange(ref ArtifactRef) bool {
	if ref.Kind == ArtifactSlideSpec {
		for _, change := range s.ChangeSet().All() {
			if change.Artifact.Key() == ref.Key() {
				return true
			}
		}
		return false
	}
	relative, err := s.resolveRelative(ref)
	if err != nil {
		return false
	}
	entry, ok := s.artifacts[relative]
	return ok && !entry.Delete
}

// Write records content in the current tool-call transaction. Repeated writes
// keep the bytes present before this tool call as the rollback preimage.
func (s *RunSession) Write(ref ArtifactRef, source string, content []byte) (ArtifactChange, error) {
	if ref.Kind == ArtifactSlideSpec {
		entries, err := spec.ReadCollection(s.ReadPath)
		if err != nil {
			return ArtifactChange{}, err
		}
		if _, err := spec.ParseStrictSourceJSON(content, "spec"); err != nil {
			return ArtifactChange{}, err
		}
		entries[ref.ID] = json.RawMessage(content)
		raw, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return ArtifactChange{}, err
		}
		before, _ := s.ReadBaseline(ref)
		if _, err := s.Write(ArtifactRef{Kind: ArtifactDerived, ID: model.SpecCollectionPath, Path: model.SpecCollectionPath}, source, append(raw, '\n')); err != nil {
			return ArtifactChange{}, err
		}
		next, err := spec.CollectionEntry(raw, ref.ID)
		if err != nil {
			return ArtifactChange{}, err
		}
		added, removed := lineDiffStat(before, next)
		return ArtifactChange{Artifact: ref, BeforeHash: hashBytes(before), AfterHash: hashBytes(next), Source: source, Insertions: added, Deletions: removed}, nil
	}
	relative, err := s.resolveRelative(ref)
	if err != nil {
		return ArtifactChange{}, err
	}
	if s.closed {
		return ArtifactChange{}, errors.New("run session is closed")
	}
	if previous, ok := s.artifacts[relative]; ok {
		previous.Source, previous.AfterContent, previous.AfterHash, previous.Delete = source, append([]byte(nil), content...), hashBytes(content), false
		s.artifacts[relative] = previous
		insertions, deletions := lineDiffStat(previous.BeforeContent, previous.AfterContent)
		return ArtifactChange{Artifact: ref, BeforeHash: previous.BeforeHash, AfterHash: previous.AfterHash, Source: source, Insertions: insertions, Deletions: deletions}, nil
	}
	before, readErr := s.readOriginal(relative)
	existed := readErr == nil
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return ArtifactChange{}, readErr
	}
	entry := sessionArtifact{
		Ref: ref, Source: source, Relative: relative,
		BeforeContent: append([]byte(nil), before...), BeforeHash: hashBytes(before),
		AfterContent: append([]byte(nil), content...), AfterHash: hashBytes(content), Existed: existed,
	}
	s.artifacts[relative] = entry
	insertions, deletions := lineDiffStat(entry.BeforeContent, entry.AfterContent)
	return ArtifactChange{Artifact: ref, BeforeHash: entry.BeforeHash, AfterHash: entry.AfterHash, Source: source, Insertions: insertions, Deletions: deletions}, nil
}

func (s *RunSession) Read(ref ArtifactRef) ([]byte, error) {
	if ref.Kind == ArtifactSlideSpec {
		return spec.ReadSlideSpec(s.ReadPath, ref.ID)
	}
	relative, err := s.resolveRelative(ref)
	if err != nil {
		return nil, err
	}
	if entry, ok := s.artifacts[relative]; ok && entry.Delete {
		return nil, fs.ErrNotExist
	}
	if entry, ok := s.artifacts[relative]; ok {
		return append([]byte(nil), entry.AfterContent...), nil
	}
	return s.readOriginal(relative)
}

func (s *RunSession) Delete(ref ArtifactRef, source string) error {
	if ref.Kind == ArtifactSlideSpec {
		entries, err := spec.ReadCollection(s.ReadPath)
		if err != nil {
			return err
		}
		delete(entries, ref.ID)
		raw, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		_, err = s.Write(ArtifactRef{Kind: ArtifactDerived, ID: model.SpecCollectionPath, Path: model.SpecCollectionPath}, source, append(raw, '\n'))
		return err
	}
	relative, err := s.resolveRelative(ref)
	if err != nil {
		return err
	}
	if s.closed {
		return errors.New("run session is closed")
	}
	entry, ok := s.artifacts[relative]
	if !ok {
		before, readErr := s.readOriginal(relative)
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		entry = sessionArtifact{Ref: ref, Relative: relative, BeforeContent: append([]byte(nil), before...), BeforeHash: hashBytes(before), Existed: readErr == nil}
	}
	entry.Source = source
	entry.AfterContent = nil
	entry.AfterHash = hashBytes(nil)
	entry.Delete = true
	s.artifacts[relative] = entry
	return nil
}

// ReadBaseline returns the bytes that existed before the current tool call
// first touched the artifact. It falls back to disk for untouched artifacts.
func (s *RunSession) ReadBaseline(ref ArtifactRef) ([]byte, error) {
	if ref.Kind == ArtifactSlideSpec {
		raw, err := s.ReadBaseline(ArtifactRef{Kind: ArtifactDerived, ID: model.SpecCollectionPath, Path: model.SpecCollectionPath})
		if err != nil {
			return nil, err
		}
		return spec.CollectionEntry(raw, ref.ID)
	}
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
	return s.readOriginal(relative)
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
		if key == model.SpecCollectionPath {
			appendSpecChanges(&out, entry)
			continue
		}
		if entry.Ref.Kind == ArtifactDerived || (!entry.Delete && entry.Existed && entry.BeforeHash == entry.AfterHash) {
			continue
		}
		change := ArtifactChange{
			Artifact: entry.Ref, BeforeHash: entry.BeforeHash,
			AfterHash: entry.AfterHash, Source: entry.Source,
		}
		change.Insertions, change.Deletions = lineDiffStat(entry.BeforeContent, entry.AfterContent)
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

func (s *RunSession) ValidateBaselines() error {
	if len(s.requiredSlides) > 0 {
		raw, err := os.ReadFile(filepath.Join(s.projectDir, ".outline.json"))
		if entry, ok := s.artifacts[".outline.json"]; ok {
			raw, err = entry.AfterContent, nil
		}
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return pptmutation.ErrSlideNotFound
			}
			return err
		}
		var outline spec.Outline
		if err := json.Unmarshal(raw, &outline); err != nil {
			return err
		}
		for id := range s.requiredSlides {
			if _, exists := spec.FindSlide(outline, id); !exists {
				return fmt.Errorf("%w: %s", pptmutation.ErrSlideNotFound, id)
			}
		}
	}
	for _, baseline := range s.unchangedResources {
		raw, err := os.ReadFile(filepath.Join(s.projectDir, baseline.ref.Path))
		if err == nil && baseline.ref.Kind == ArtifactSlideSpec {
			raw, err = spec.CollectionEntry(raw, baseline.ref.ID)
		}
		if err != nil || resourceContentHash(baseline.resource, raw) != baseline.hash {
			return fmt.Errorf("%w: %s", ErrArtifactHashMismatch, baseline.ref.Path)
		}
	}
	for path, entry := range s.artifacts {
		raw, err := os.ReadFile(filepath.Join(s.projectDir, entry.Relative))
		if errors.Is(err, fs.ErrNotExist) && !entry.Existed {
			continue
		}
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("%w: %s was removed", ErrArtifactHashMismatch, path)
			}
			return err
		}
		if hashBytes(raw) != entry.BeforeHash {
			structured := entry.Source == "edit_manifest" || entry.Source == "edit_design" || entry.Source == "edit_spec"
			if !structured || spec.ResourceBytesHash(raw) == "" || spec.ResourceBytesHash(raw) != spec.ResourceBytesHash(entry.BeforeContent) {
				return fmt.Errorf("%w: %s", ErrArtifactHashMismatch, path)
			}
			// Rebase the physical preimage only after semantic equality is
			// established. Rollback must preserve the current file's layout.
			entry.BeforeContent, entry.BeforeHash = append([]byte(nil), raw...), hashBytes(raw)
			s.artifacts[path] = entry
		}
	}
	return nil
}

func (s *RunSession) CommitOperation(ctx context.Context, operationID, toolResultJSON string, metadata CommitMetadata) (ChangeSet, error) {
	if s.closed {
		return EmptyChangeSet(), errors.New("run session is closed")
	}
	if strings.TrimSpace(operationID) == "" {
		return EmptyChangeSet(), errors.New("operation id is required")
	}
	release := pptmutation.LockProject(s.projectDir)
	defer release()
	if err := ctx.Err(); err != nil {
		s.resetOperation()
		return EmptyChangeSet(), err
	}
	if err := s.ValidateBaselines(); err != nil {
		s.resetOperation()
		return EmptyChangeSet(), err
	}
	changes := s.ChangeSet()
	snapshot := s.Snapshot()
	if snapshot == nil {
		return EmptyChangeSet(), errors.New("run session is closed")
	}
	requestHash, err := mutationRequestHash(s.runID, operationID, snapshot.Artifacts, s.generationInputs)
	if err != nil {
		return EmptyChangeSet(), err
	}
	journal := MutationJournal{
		RunID: s.runID, OperationID: operationID, RequestHash: requestHash,
		Artifacts: snapshot.Artifacts, GenerationInputs: s.generationInputs,
	}
	journalPath := mutationJournalPath(s.projectDir, s.runID, operationID)
	if err := writeMutationJournal(journalPath, journal); err != nil {
		s.resetOperation()
		return EmptyChangeSet(), err
	}
	keys := make([]string, 0, len(s.artifacts))
	for key := range s.artifacts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rollback := func(cause error) error {
		if rollbackErr := restoreMutationArtifacts(s.projectDir, journal.Artifacts, false); rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("mutation rollback failed: %w", rollbackErr))
		}
		_ = removeMutationJournal(journalPath)
		return cause
	}
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			s.resetOperation()
			return EmptyChangeSet(), rollback(err)
		}
		entry := s.artifacts[key]
		path := filepath.Join(s.projectDir, entry.Relative)
		var err error
		if entry.Delete {
			err = durableRemove(path)
		} else {
			err = atomicWrite(path, entry.AfterContent)
		}
		if err != nil {
			s.resetOperation()
			return EmptyChangeSet(), rollback(err)
		}
	}
	if metadata != nil {
		if err := ctx.Err(); err != nil {
			s.resetOperation()
			return EmptyChangeSet(), rollback(err)
		}
		commitContext := CommitContext{
			OperationID:      operationID,
			RequestHash:      journal.RequestHash,
			ToolResultJSON:   toolResultJSON,
			Changes:          s.ChangeSet(),
			GenerationInputs: s.generationInputs,
		}
		if err := metadata(ctx, commitContext); err != nil {
			s.resetOperation()
			return EmptyChangeSet(), rollback(err)
		}
	}
	journal.Committed = true
	_ = writeMutationJournal(journalPath, journal)
	_ = removeMutationJournal(journalPath)
	s.resetOperation()
	return changes, nil
}

func mutationRequestHash(runID, operationID string, artifacts []RunSessionArtifactState, inputs map[string]json.RawMessage) (string, error) {
	hashInput, err := json.Marshal(struct {
		RunID       string                     `json:"run_id"`
		OperationID string                     `json:"operation_id"`
		Artifacts   []RunSessionArtifactState  `json:"artifacts"`
		Inputs      map[string]json.RawMessage `json:"generation_inputs,omitempty"`
	}{runID, operationID, artifacts, inputs})
	if err != nil {
		return "", err
	}
	return hashBytes(hashInput), nil
}

// Commit is retained as the explicit close operation for callers outside the
// Runtime. Runtime tool calls use CommitOperation and keep the session open.
func (s *RunSession) Commit(ctx context.Context, metadata CommitMetadata) error {
	if _, err := s.CommitOperation(ctx, "session_commit", "", metadata); err != nil {
		return err
	}
	s.closed = true
	activeRunSessions.CompareAndDelete(s.projectDir, s)
	return nil
}

func mutationJournalPath(projectDir, runID, operationID string) string {
	name := hashBytes([]byte(runID+"\x00"+operationID)) + ".json"
	return filepath.Join(projectDir, ".runtime", "mutations", name)
}

func writeMutationJournal(path string, journal MutationJournal) error {
	raw, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	return atomicWrite(path, raw)
}

func removeMutationJournal(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func restoreMutationArtifacts(projectDir string, artifacts []RunSessionArtifactState, committed bool) error {
	for index := len(artifacts) - 1; index >= 0; index-- {
		entry := artifacts[index]
		relative := filepath.Clean(filepath.FromSlash(entry.Relative))
		if relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return ErrInvalidArtifactPath
		}
		if hashBytes(entry.BeforeContent) != entry.BeforeHash || hashBytes(entry.AfterContent) != entry.AfterHash {
			return errors.New("mutation journal hash mismatch")
		}
		path := filepath.Join(projectDir, relative)
		current, readErr := os.ReadFile(path)
		currentExists := readErr == nil
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		matchesBefore := currentExists == entry.Existed && (!currentExists || hashBytes(current) == entry.BeforeHash)
		postExists := !entry.Delete
		matchesAfter := currentExists == postExists && (!currentExists || hashBytes(current) == entry.AfterHash)
		if !matchesBefore && !matchesAfter {
			if committed {
				// A later committed operation or an intentional user edit has
				// superseded this already-receipted postimage. Preserve it.
				continue
			}
			return fmt.Errorf("%w: recovery target %s changed outside the journal", ErrArtifactHashMismatch, entry.Relative)
		}
		if committed {
			if matchesAfter {
				continue
			}
			if entry.Delete {
				if err := durableRemove(path); err != nil {
					return err
				}
				continue
			}
			if err := atomicWrite(path, entry.AfterContent); err != nil {
				return err
			}
			continue
		}
		if matchesBefore {
			continue
		}
		if entry.Existed {
			if err := atomicWrite(path, entry.BeforeContent); err != nil {
				return err
			}
		} else if err := durableRemove(path); err != nil {
			return err
		}
	}
	return nil
}

func RecoverMutationJournals(ctx context.Context, projectDir string, lookup MutationReceiptLookup) error {
	release := pptmutation.LockProject(projectDir)
	defer release()
	dir := filepath.Join(projectDir, ".runtime", "mutations")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var journal MutationJournal
		if err := json.Unmarshal(raw, &journal); err != nil {
			return err
		}
		if strings.TrimSpace(journal.RunID) == "" || strings.TrimSpace(journal.OperationID) == "" {
			return errors.New("mutation journal has no operation identity")
		}
		expectedHash, hashErr := mutationRequestHash(journal.RunID, journal.OperationID, journal.Artifacts, journal.GenerationInputs)
		if hashErr != nil {
			return hashErr
		}
		if expectedHash != journal.RequestHash {
			return errors.New("mutation journal request hash mismatch")
		}
		committed := journal.Committed
		if !committed && lookup != nil {
			receiptHash, found, lookupErr := lookup(ctx, journal.RunID, journal.OperationID)
			if lookupErr != nil {
				return lookupErr
			}
			if found {
				if receiptHash != journal.RequestHash {
					return errors.New("mutation receipt hash mismatch")
				}
				committed = true
			}
		}
		if err := restoreMutationArtifacts(projectDir, journal.Artifacts, committed); err != nil {
			return err
		}
		if err := removeMutationJournal(path); err != nil {
			return err
		}
	}
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
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func durableRemove(path string) error {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum)
}
