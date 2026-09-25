package workflow

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"sync"
)

type checkpointLeaseKey struct{}

// CheckpointLease serializes checkpoint commits belonging to one worker. The
// database still checks all versions; this token never grants a new ownership.
type CheckpointLease struct {
	mu        sync.Mutex
	Owner     string
	Execution int64
	Revision  int64
	Scope     model.RunScope
}

func WithCheckpointLease(ctx context.Context, owner string, execution, revision int64) context.Context {
	return context.WithValue(ctx, checkpointLeaseKey{}, &CheckpointLease{Owner: owner, Execution: execution, Revision: revision})
}
func WithCheckpointWrite(ctx context.Context, cp RuntimeCheckpoint, write func(RuntimeCheckpoint) error) error {
	lease, _ := ctx.Value(checkpointLeaseKey{}).(*CheckpointLease)
	if lease == nil {
		return write(cp)
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	cp.OwnerInstanceID = lease.Owner
	cp.ExecutionRevision = lease.Execution
	cp.CheckpointRevision = lease.Revision
	if lease.Scope.Revision > cp.Scope.Revision {
		cp.Scope = lease.Scope
	}
	if err := write(cp); err != nil {
		return err
	}
	lease.Revision++
	return nil
}

func CheckpointOwnership(ctx context.Context) (string, int64, bool) {
	lease, ok := ctx.Value(checkpointLeaseKey{}).(*CheckpointLease)
	if !ok {
		return "", 0, false
	}
	return lease.Owner, lease.Execution, true
}

// WithExternalCheckpointWrite serializes accepted steering with worker checkpoints.
// The callback returns the committed checkpoint, including its canonical scope.
func WithExternalCheckpointWrite(ctx context.Context, write func() (RuntimeCheckpoint, error)) error {
	lease, _ := ctx.Value(checkpointLeaseKey{}).(*CheckpointLease)
	if lease == nil {
		_, err := write()
		return err
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	cp, err := write()
	if err != nil {
		return err
	}
	if cp.ExecutionRevision != lease.Execution || cp.OwnerInstanceID != lease.Owner {
		return context.Canceled
	}
	lease.Revision = cp.CheckpointRevision
	lease.Scope = cp.Scope
	return nil
}

// ShareCheckpointLease preserves the request cancellation while sharing worker authority.
func ShareCheckpointLease(ctx, worker context.Context) context.Context {
	if lease, ok := worker.Value(checkpointLeaseKey{}).(*CheckpointLease); ok {
		return context.WithValue(ctx, checkpointLeaseKey{}, lease)
	}
	return ctx
}
