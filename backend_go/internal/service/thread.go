package service

import (
	"context"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/google/uuid"
)

// ThreadService manages conversation metadata and current session views.
type ThreadService struct {
	store     store.Store
	clock     func() int64
	newID     func() string
	deletions *DeletionService
}

type CreateThreadParams struct {
	Title string
}

func NewThreadService(s store.Store) *ThreadService {
	return &ThreadService{
		store: s, clock: func() int64 { return time.Now().UnixMilli() }, newID: uuid.NewString,
	}
}

func (svc *ThreadService) CreateThread(ctx context.Context, projectID string, p CreateThreadParams) (model.Thread, error) {
	_, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return model.Thread{}, err
	}
	id := svc.newID()
	now := svc.clock()
	title := strings.TrimSpace(p.Title)
	if title != "" {
		title, err = ValidateThreadTitle(title)
		if err != nil {
			return model.Thread{}, err
		}
	}
	th := model.Thread{
		ID:                     id,
		ProjectID:              projectID,
		Title:                  title,
		AutoRenameEnabled:      title == "",
		AutoSuggestEnabled:     true,
		SuggestVersion:         1,
		NamingRevision:         1,
		RenameOperationVersion: 1,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := svc.store.CreateThread(ctx, th); err != nil {
		return model.Thread{}, err
	}
	return th, nil
}

func (svc *ThreadService) ListThreads(ctx context.Context, projectID string) ([]model.Thread, error) {
	if _, err := svc.store.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	return svc.store.ListThreads(ctx, projectID)
}

func (svc *ThreadService) GetThread(ctx context.Context, id string) (model.Thread, error) {
	return svc.store.GetThread(ctx, id)
}

func (svc *ThreadService) ClearPlan(ctx context.Context, id string, revision int64, requestKey string) (sessionstate.Receipt, error) {
	return svc.store.ClearThreadPlan(ctx, id, revision, requestKey)
}

func (svc *ThreadService) DeleteThread(ctx context.Context, id string) (store.Deletion, error) {
	return svc.deletions.DeleteThread(ctx, id)
}

func (svc *ThreadService) ReadState() sessionstate.Reader { return svc.store }

func (svc *ThreadService) CommandStore() CommandStore {
	value, _ := svc.store.(CommandStore)
	return value
}
