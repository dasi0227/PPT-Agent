package service

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type deletionJob struct {
	wake chan struct{}
	done chan struct{}
}

// Receipts are the durable authority for the undo deadline and cleanup queue.
// Local workers only stop writers and execute the receipt's current decision.
type DeletionService struct {
	store  store.Store
	root   string
	log    *zap.Logger
	stop   func(context.Context, store.Deletion) error
	gate   func(string) func()
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	jobs   map[string]*deletionJob
	closed bool
	wg     sync.WaitGroup
}

func NewDeletionService(projects *ProjectService, threads *ThreadService, root string, log *zap.Logger, stop func(context.Context, store.Deletion) error, gate func(string) func()) *DeletionService {
	ctx, cancel := context.WithCancel(context.Background())
	svc := &DeletionService{store: projects.store, root: root, log: log, stop: stop, gate: gate, ctx: ctx, cancel: cancel, jobs: map[string]*deletionJob{}}
	projects.deletions, threads.deletions = svc, svc
	return svc
}

func (svc *DeletionService) DeleteProject(ctx context.Context, id string) (store.Deletion, error) {
	if err := svc.store.DeleteProject(ctx, id); err != nil {
		return store.Deletion{}, err
	}
	return svc.accept(ctx, "project", id)
}

func (svc *DeletionService) DeleteThread(ctx context.Context, id string) (store.Deletion, error) {
	if err := svc.store.DeleteThread(ctx, id); err != nil {
		return store.Deletion{}, err
	}
	return svc.accept(ctx, "thread", id)
}

func (svc *DeletionService) accept(ctx context.Context, kind, id string) (store.Deletion, error) {
	target, err := svc.store.GetDeletion(ctx, kind, id)
	if err == nil && target.Status != "completed" {
		svc.schedule(target)
	}
	return target, err
}

func (svc *DeletionService) Pending(ctx context.Context) ([]store.Deletion, error) {
	jobs, err := svc.store.PendingDeletions(ctx)
	if err != nil {
		return nil, err
	}
	out := []store.Deletion{}
	projects := map[string]bool{}
	for _, target := range jobs {
		if target.Kind == "project" {
			projects[target.ProjectID] = true
		}
	}
	for _, target := range jobs {
		if target.Status == "pending" && target.UndoUntil > time.Now().UnixMilli() && (target.Kind == "project" || !projects[target.ProjectID]) {
			out = append(out, target)
		}
	}
	return out, nil
}

func (svc *DeletionService) Restore(ctx context.Context, kind, id, token string) (store.Deletion, error) {
	target, err := svc.store.GetDeletion(ctx, kind, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Repeating an acknowledged undo is harmless, including after a lost HTTP
		// response. A later deletion still has a distinct receipt and token.
		if kind == "project" {
			if project, getErr := svc.store.GetProject(ctx, id); getErr == nil {
				return store.Deletion{Kind: kind, ID: id, ProjectID: id, Title: project.Title, Token: token}, nil
			}
		} else {
			if thread, getErr := svc.store.GetThread(ctx, id); getErr == nil {
				return store.Deletion{Kind: kind, ID: id, ProjectID: thread.ProjectID, Title: thread.Title, Token: token}, nil
			}
		}
		return target, store.ErrDeletionExpired
	}
	if err != nil {
		return target, err
	}
	if target.Token != token {
		return target, store.ErrDeletionExpired
	}
	if err := svc.store.RequestDeletionRestore(ctx, target); err != nil {
		return target, err
	}
	job := svc.schedule(target)
	if job == nil {
		return target, errors.New("deletion service is shutting down")
	}
	select {
	case <-job.done:
	case <-ctx.Done():
		return target, ctx.Err()
	}
	current, err := svc.store.GetDeletion(ctx, kind, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return target, nil
	}
	if err != nil {
		return target, err
	}
	if current.Token != token {
		return target, nil
	}
	return target, errors.New("deletion restoration did not finish")
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

func (svc *DeletionService) Shutdown(ctx context.Context) error {
	svc.mu.Lock()
	svc.closed = true
	svc.cancel()
	svc.mu.Unlock()
	done := make(chan struct{})
	go func() { svc.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (svc *DeletionService) schedule(target store.Deletion) *deletionJob {
	key := target.Kind + ":" + target.ID + ":" + target.Token
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.closed {
		return nil
	}
	if job := svc.jobs[key]; job != nil {
		select {
		case job.wake <- struct{}{}:
		default:
		}
		return job
	}
	job := &deletionJob{wake: make(chan struct{}, 1), done: make(chan struct{})}
	svc.jobs[key] = job
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		defer func() { svc.mu.Lock(); delete(svc.jobs, key); close(job.done); svc.mu.Unlock() }()
		stopped := false
		for delay := time.Second; svc.ctx.Err() == nil; {
			current, err := svc.store.GetDeletion(svc.ctx, target.Kind, target.ID)
			if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (current.Token != target.Token || current.Status == "completed")) {
				return
			}
			if err == nil && !stopped {
				err = svc.stop(svc.ctx, current)
				if err == nil {
					release := svc.gate(current.ProjectID)
					err = svc.store.QuiesceDeletion(svc.ctx, current)
					release()
					stopped = err == nil
				}
			}
			if err == nil && current.Status == "pending" && current.UndoUntil > time.Now().UnixMilli() {
				if !svc.wait(job, time.Until(time.UnixMilli(current.UndoUntil))) {
					return
				}
				continue
			}
			if err == nil {
				err = svc.settle(current)
			}
			if err == nil {
				return
			}
			if svc.ctx.Err() != nil {
				return
			}
			svc.log.Warn("deletion cleanup deferred", zap.String("kind", target.Kind), zap.String("id", target.ID), zap.Error(err))
			if !svc.wait(job, delay) {
				return
			}
			delay = min(delay*2, 30*time.Second)
		}
	}()
	return job
}

func (svc *DeletionService) wait(job *deletionJob, duration time.Duration) bool {
	timer := time.NewTimer(max(0, duration))
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-job.wake:
		return true
	case <-svc.ctx.Done():
		return false
	}
}

func (svc *DeletionService) settle(target store.Deletion) error {
	release := svc.gate(target.ProjectID)
	defer release()
	releaseFiles := pptmutation.LockProject(filepath.Join(svc.root, "projects", target.ProjectID, "artifacts"))
	defer releaseFiles()
	current, err := svc.store.GetDeletion(svc.ctx, target.Kind, target.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.Token != target.Token {
		return nil
	}
	if current.Status == "restoring" {
		if err := svc.store.FinishDeletionRestore(svc.ctx, current); err != nil {
			return err
		}
		return svc.store.FlushProjectEvents(svc.ctx, current.ProjectID)
	}
	claimed, err := svc.store.FinalizeDeletion(svc.ctx, current)
	if err != nil {
		return err
	}
	if !claimed {
		// Undo may have claimed the receipt while this worker waited for the gate.
		current, err = svc.store.GetDeletion(svc.ctx, target.Kind, target.ID)
		if err != nil {
			return err
		}
		if current.Token == target.Token && current.Status == "restoring" {
			return svc.store.FinishDeletionRestore(svc.ctx, current)
		}
		return nil
	}
	if err := svc.store.PurgeDeletion(svc.ctx, current); err != nil {
		return err
	}
	return svc.store.CompleteDeletion(svc.ctx, current)
}
