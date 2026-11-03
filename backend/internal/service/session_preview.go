package service

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func (svc *ThreadService) TimelinePreview(ctx context.Context, threadID, itemID string) (*model.PreviewAssessment, error) {
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return nil, sessionstate.ErrSceneChanged
	}
	current, err := svc.store.ThreadSceneRevision(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if current != scene {
		return nil, sessionstate.ErrSceneChanged
	}
	assessment, err := svc.store.TimelinePreview(ctx, threadID, itemID)
	if err != nil {
		return nil, err
	}
	if assessment != nil {
		thread, err := svc.store.GetThread(ctx, threadID)
		if err != nil {
			return nil, err
		}
		project, err := svc.store.GetProject(ctx, thread.ProjectID)
		if err != nil {
			return nil, err
		}
		assessment = workflow.CurrentPreview(ctx, project, *assessment)
	}
	current, err = svc.store.ThreadSceneRevision(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if current != scene {
		return nil, sessionstate.ErrSceneChanged
	}
	return assessment, nil
}
