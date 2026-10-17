package service

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"go.uber.org/zap"
)

// DeletionService accepts deletion independently of workers and history gates.
// Only physical cleanup waits for canceled writers; receipts survive restarts.
type DeletionService struct {
	store   store.Store
	root    string
	log     *zap.Logger
	workers sync.Map
	stop    func(context.Context, store.Deletion) error
	gate    func(string) func()
}

func NewDeletionService(projects *ProjectService, threads *ThreadService, root string, log *zap.Logger, stop func(context.Context, store.Deletion) error, gate func(string) func()) *DeletionService {
	svc := &DeletionService{store: projects.store, root: root, log: log, stop: stop, gate: gate}
	projects.deletions, threads.deletions = svc, svc
	return svc
}

func (svc *DeletionService) DeleteProject(ctx context.Context, id string) error {
	if err := svc.store.DeleteProject(ctx, id); err != nil {
		return err
	}
	svc.schedule(store.Deletion{Kind: "project", ID: id, ProjectID: id})
	return nil
}

func (svc *DeletionService) DeleteThread(ctx context.Context, id string) error {
	// The receipt is also used for idempotent repeats after the thread is gone.
	if err := svc.store.DeleteThread(ctx, id); err != nil {
		return err
	}
	svc.schedule(store.Deletion{Kind: "thread", ID: id})
	return nil
}

func (svc *DeletionService) Recover(ctx context.Context) error {
	jobs, err := svc.store.PendingDeletions(ctx)
	if err != nil {
		return err
	}
	for _, target := range jobs {
		svc.schedule(target)
	}
	return nil
}

func (svc *DeletionService) schedule(target store.Deletion) {
	key := target.Kind + ":" + target.ID
	if _, loaded := svc.workers.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	go func() {
		defer svc.workers.Delete(key)
		for delay := time.Second; ; {
			if err := svc.cleanup(target); err == nil {
				return
			} else {
				svc.log.Warn("deletion cleanup deferred", zap.String("kind", target.Kind), zap.String("id", target.ID), zap.Error(err))
			}
			time.Sleep(delay)
			if delay < 30*time.Second {
				delay = min(delay*2, 30*time.Second)
			}
		}
	}()
}

func (svc *DeletionService) cleanup(target store.Deletion) error {
	ctx := context.Background()
	if target.ProjectID == "" {
		receipt, err := svc.store.GetIdempotency(ctx, "delete", target.ID, target.Kind)
		if err != nil {
			return err
		}
		target.ProjectID = receipt.RequestHash
	}
	if err := svc.stop(ctx, target); err != nil {
		return err
	}
	// Existing history requests can finish, but cannot put deleted rows back.
	// Keep any final file restore ahead of physical deletion.
	release := svc.gate(target.ProjectID)
	defer release()
	if err := svc.stop(ctx, target); err != nil {
		return err
	}
	releaseFiles := pptmutation.LockProject(filepath.Join(svc.root, "projects", target.ProjectID, "artifacts"))
	defer releaseFiles()
	if err := svc.store.PurgeDeletion(ctx, target); err != nil {
		return err
	}
	return svc.store.CompleteDeletion(ctx, target)
}
