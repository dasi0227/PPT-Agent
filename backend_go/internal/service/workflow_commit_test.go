package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type artifactCommitFaultStore struct {
	store.Store
	attempted bool
	commit    func(context.Context, model.ArtifactCommit) error
	lookup    func(context.Context, string, string, string) (model.IdempotencyRecord, error)
}

func (s *artifactCommitFaultStore) CommitWorkflow(ctx context.Context, commit model.ArtifactCommit) error {
	s.attempted = true
	return s.commit(ctx, commit)
}

func (s *artifactCommitFaultStore) GetIdempotency(ctx context.Context, scope, owner, key string) (model.IdempotencyRecord, error) {
	if s.attempted && s.lookup != nil {
		return s.lookup(ctx, scope, owner, key)
	}
	return s.Store.GetIdempotency(ctx, scope, owner, key)
}

func TestWorkflowCommitReconcilesLostAcknowledgementBeforeChangingFiles(t *testing.T) {
	for _, outcome := range []string{"committed", "not_committed", "lookup_failed", "conflicting_hash", "incomplete_receipt"} {
		t.Run(outcome, func(t *testing.T) {
			f := newServiceFixture(t)
			ctx, cancel := context.WithCancel(workflow.WithCheckpointLease(sessionstate.WithRequestScene(context.Background(), 1), "test", 1, 0))
			defer cancel()
			if err := f.store.CreateRun(ctx, model.Run{ID: "run", ThreadID: f.thread.ID, ProjectID: f.project.ID, SceneRevision: 1, ClientRequestID: "commit-input", OwnerInstanceID: "test", Status: model.RunRunning,
				Command: model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeCreate, Instruction: "test commit"}}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(f.project.WorkDir, "notes.txt")
			if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
				t.Fatal(err)
			}
			session, err := workflow.NewRunSession(f.project.WorkDir, "run")
			if err != nil {
				t.Fatal(err)
			}
			defer session.Discard()
			if _, err := session.Write(workflow.ArtifactRef{Kind: workflow.ArtifactProjectFile, ID: "notes.txt", Path: "notes.txt"}, "test", []byte("after")); err != nil {
				t.Fatal(err)
			}
			commitErr := errors.New("commit acknowledgement lost")
			lookupErr := errors.New("receipt query unavailable")
			st := &artifactCommitFaultStore{Store: f.store}
			st.commit = func(ctx context.Context, commit model.ArtifactCommit) error {
				if outcome != "not_committed" {
					if err := f.store.CommitWorkflow(ctx, commit); err != nil {
						t.Fatal(err)
					}
				}
				cancel()
				return errors.Join(commitErr, context.Canceled)
			}
			lookups := 0
			st.lookup = func(ctx context.Context, scope, owner, key string) (model.IdempotencyRecord, error) {
				lookups++
				if ctx.Err() != nil || scope != "artifact_commit" || owner != "run" || key != "write" {
					t.Fatalf("receipt lookup lost its context or identity: %v %s %s %s", ctx.Err(), scope, owner, key)
				}
				if _, bounded := ctx.Deadline(); !bounded {
					t.Fatal("receipt lookup has no deadline")
				}
				if outcome == "lookup_failed" {
					return model.IdempotencyRecord{}, lookupErr
				}
				receipt, err := f.store.GetIdempotency(ctx, scope, owner, key)
				if outcome == "conflicting_hash" {
					receipt.RequestHash = "another-operation-payload"
				}
				if outcome == "incomplete_receipt" {
					receipt.Status = "in_progress"
				}
				return receipt, err
			}
			committer := workflowCommitter{store: st, project: f.project, runID: "run"}
			changes, err := session.CommitOperation(ctx, "write", "", committer.Commit)
			if lookups != 1 {
				t.Fatalf("receipt lookups=%d", lookups)
			}
			unknown := outcome != "committed" && outcome != "not_committed"
			if outcome == "committed" {
				if err != nil || changes.Count() != 1 {
					t.Fatalf("committed operation became a failure: %+v %v", changes, err)
				}
			} else if !errors.Is(err, commitErr) || errors.Is(err, pptmutation.ErrRecoveryRequired) != unknown {
				t.Fatalf("wrong commit classification: %v", err)
			}
			if outcome == "lookup_failed" && !errors.Is(err, lookupErr) {
				t.Fatalf("lookup cause was lost: %v", err)
			}
			want := "after"
			if outcome == "not_committed" {
				want = "before"
			}
			if raw, err := os.ReadFile(path); err != nil || string(raw) != want {
				t.Fatalf("file=%q want=%q err=%v", raw, want, err)
			}
			entries, readErr := os.ReadDir(filepath.Join(f.project.WorkDir, ".runtime", "mutations"))
			if readErr != nil || (len(entries) != 0) != unknown {
				t.Fatalf("pending journal mismatch: %v %v", entries, readErr)
			}
			if unknown {
				if _, err := workflow.NewRunSession(f.project.WorkDir, "another-run"); !errors.Is(err, pptmutation.ErrRecoveryRequired) {
					t.Fatalf("unknown outcome did not fence new work: %v", err)
				}
				if err := (&RunService{store: f.store}).recoverProjectMutations(context.Background(), f.project); err != nil {
					t.Fatal(err)
				}
				if raw, err := os.ReadFile(path); err != nil || string(raw) != "after" {
					t.Fatalf("receipt recovery rewound committed bytes: %q %v", raw, err)
				}
			}
		})
	}
}

func TestPendingArtifactJournalFencesManualWritesAfterRunCancellation(t *testing.T) {
	f := newServiceFixture(t)
	ctx := context.Background()
	if err := f.store.CreateRun(ctx, model.Run{ID: "run", ThreadID: f.thread.ID, ProjectID: f.project.ID, SceneRevision: 1, ClientRequestID: "commit-input", Status: model.RunRunning,
		Command: model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeCreate, Instruction: "test commit"}}); err != nil {
		t.Fatal(err)
	}
	session, err := workflow.NewRunSession(f.project.WorkDir, "run")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	if _, err := session.Write(workflow.ArtifactRef{Kind: workflow.ArtifactProjectFile, ID: "notes.txt", Path: "notes.txt"}, "test", []byte("after")); err != nil {
		t.Fatal(err)
	}
	if _, err := session.CommitOperation(ctx, "write", "", func(context.Context, workflow.CommitContext) error { return pptmutation.ErrRecoveryRequired }); !errors.Is(err, pptmutation.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	if err := f.store.SetRunStatus(ctx, "run", model.RunCanceled); err != nil {
		t.Fatal(err)
	}
	if active, err := f.store.HasActiveRun(ctx, f.project.ID); err != nil || active {
		t.Fatalf("canceled Run still fences the test: %v %v", active, err)
	}
	if _, _, err := NewPPTMutationServiceWithLocks(f.store, f.locks).Apply(ctx, f.project.ID, pptmutation.Request{Op: "meta.create"}); !errors.Is(err, pptmutation.ErrRecoveryRequired) {
		t.Fatalf("manual page edit bypassed pending journal: %v", err)
	}
	appearance := NewProjectService(f.store, WorkRoot(filepath.Join(f.project.WorkDir, "..", "..", "..")))
	if _, release, err := appearance.lockAppearance(ctx, f.project.ID); !errors.Is(err, pptmutation.ErrRecoveryRequired) {
		if release != nil {
			release()
		}
		t.Fatalf("appearance edit bypassed pending journal: %v", err)
	}
	if _, err := NewAttachmentServiceWithLocks(f.store, f.locks).Upload(ctx, f.project.ID, "blocked.png", strings.NewReader("")); !errors.Is(err, pptmutation.ErrRecoveryRequired) {
		t.Fatalf("attachment upload bypassed pending journal: %v", err)
	}
	if err := (&RunService{store: f.store}).recoverProjectMutations(ctx, f.project); err != nil {
		t.Fatal(err)
	}
	if _, release, err := appearance.lockAppearance(ctx, f.project.ID); err != nil {
		t.Fatalf("completed recovery did not release the fence: %v", err)
	} else {
		release()
	}
}
