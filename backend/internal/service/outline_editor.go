package service

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/outline"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// outlineEditorAdapter 把 SlideService 的无互斥核心暴露给 AI 大纲编辑 runner。
// AI run 本身即活跃 run，故走 *Core 方法（不再重复 RUN_ACTIVE 判定），
// 与手动 REST 共享同一底层落库/落盘逻辑（DRY）。
type outlineEditorAdapter struct {
	svc       *SlideService
	projectID string
}

var _ outline.OutlineEditor = (*outlineEditorAdapter)(nil)

func (a *outlineEditorAdapter) ListSlides(ctx context.Context, projectID string) ([]model.Slide, error) {
	return a.svc.store.ListSlides(ctx, projectID)
}

func (a *outlineEditorAdapter) PatchOutline(ctx context.Context, slideID string, p outline.OutlinePatch) (slidejson.SlideJSON, error) {
	sl, err := a.svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	// R0 projects store a semantic Blueprint v2 in slide.json. Keep the
	// legacy outline agent as an implementation detail, but translate its
	// field-level patch onto the v2 aggregate so it can never downgrade the
	// file back to the legacy slide-json shape.
	bpSvc := NewBlueprintService(a.svc.store)
	if current, _, bpErr := bpSvc.GetSlide(ctx, slideID); bpErr == nil && current.SchemaVersion == blueprint.SchemaVersion {
		if p.Title != nil {
			current.Title = *p.Title
		}
		if p.Subtitle != nil {
			current.KeyMessage = *p.Subtitle
		}
		if p.ContentIntent != nil {
			current.Content.Summary = *p.ContentIntent
		}
		if p.Bullets != nil {
			current.Content.Points = append([]string(nil), (*p.Bullets)...)
		}
		if p.Layout != nil {
			current.VisualIntent.Archetype = *p.Layout
		}
		updated, patchErr := bpSvc.PatchSlide(ctx, slideID, current.Revision, current)
		if patchErr != nil {
			return slidejson.SlideJSON{}, patchErr
		}
		return slidejson.SlideJSON{
			Title:         updated.Title,
			Subtitle:      updated.KeyMessage,
			ContentIntent: updated.Content.Summary,
			Bullets:       append([]string(nil), updated.Content.Points...),
			Layout:        updated.VisualIntent.Archetype,
			Notes:         updated.SpeakerNotes,
		}, nil
	}
	return a.svc.patchContentCore(ctx, sl, SlidePatch{
		Title: p.Title, Subtitle: p.Subtitle, ContentIntent: p.ContentIntent,
		Layout: p.Layout, Bullets: p.Bullets,
	})
}

func (a *outlineEditorAdapter) AddOutline(ctx context.Context, projectID, afterSlideID, layout string) (model.Slide, error) {
	return a.svc.addSlideCore(ctx, projectID, afterSlideID, layout)
}

func (a *outlineEditorAdapter) DeleteOutline(ctx context.Context, slideID string) error {
	sl, err := a.svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return err
	}
	return a.svc.deleteSlideCore(ctx, sl)
}

func (a *outlineEditorAdapter) ReorderOutline(ctx context.Context, projectID string, orderedIDs []string) error {
	return a.svc.reorderSlidesCore(ctx, projectID, orderedIDs)
}

// NewOutlineEditor 构造大纲编辑适配器（供 run.go 装配 AI 大纲编辑 runner）。
func NewOutlineEditor(svc *SlideService, projectID string) outline.OutlineEditor {
	return &outlineEditorAdapter{svc: svc, projectID: projectID}
}
