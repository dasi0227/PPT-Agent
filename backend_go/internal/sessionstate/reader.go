package sessionstate

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Reader exposes only committed current views and bounded continuation pages.
type Reader interface {
	SessionView(context.Context, string, int) (View, error)
	Timeline(context.Context, string, *PageCursor, int) (TimelinePage, error)
	TimelineItems(context.Context, string, string, *PageCursor, int) (ItemPage, error)
	TimelineContent(context.Context, string, string, ContentRef) (json.RawMessage, error)
	TimelinePreview(context.Context, string, string) (*model.PreviewAssessment, error)
	SessionChanges(context.Context, Cursor, int) (ChangePage, error)
	SubscribeSession(string) (<-chan struct{}, func())
}
