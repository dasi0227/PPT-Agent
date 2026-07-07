package service

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ReadContent 读某页 slide.json 全文；文件缺失时用元数据回退最小结构。
func (svc *SlideService) ReadContent(ctx context.Context, slideID string) (slidejson.SlideJSON, error) {
	sl, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	proj, err := svc.store.GetProject(ctx, sl.ProjectID)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	sb, err := tools.NewSandbox(proj.WorkDir)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	raw, err := sb.Read(model.SlideJSONPath(slideID))
	if err != nil {
		return slidejson.SlideJSON{ID: sl.ID, Idx: sl.Idx, Layout: sl.Layout, Title: sl.Title}, nil
	}
	return slidejson.Parse(raw)
}
