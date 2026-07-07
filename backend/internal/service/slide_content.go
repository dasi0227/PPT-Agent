package service

import (
	"context"
	"encoding/json"

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

// SlidePatch 是字段级局部更新载荷；nil 字段表示不改。
type SlidePatch struct {
	Title         *string
	Subtitle      *string
	ContentIntent *string
	Layout        *string
	Bullets       *[]string
	ChartIntent   *slidejson.ChartIntent
	Steps         *int
}

// PatchContent 局部更新某页 slide.json（回写 + 校验），不产版本快照。
//   - 改了 title/layout 则同步 DB 元数据；
//   - 若该页有 html（大纲已改但产物未同步）则置 outline_dirty=true。
func (svc *SlideService) PatchContent(ctx context.Context, slideID string, p SlidePatch) (slidejson.SlideJSON, error) {
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

	cur, err := svc.ReadContent(ctx, slideID)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}

	if p.Title != nil {
		cur.Title = *p.Title
	}
	if p.Subtitle != nil {
		cur.Subtitle = *p.Subtitle
	}
	if p.ContentIntent != nil {
		cur.ContentIntent = *p.ContentIntent
	}
	if p.Layout != nil {
		cur.Layout = *p.Layout
	}
	if p.Bullets != nil {
		cur.Bullets = *p.Bullets
	}
	if p.ChartIntent != nil {
		cur.ChartIntent = p.ChartIntent
	}
	if p.Steps != nil {
		cur.Steps = *p.Steps
	}
	cur.ID, cur.Idx = sl.ID, sl.Idx

	if err := slidejson.Validate(cur); err != nil {
		return slidejson.SlideJSON{}, validationError("%s", err.Error())
	}
	raw, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	if err := sb.Write(model.SlideJSONPath(slideID), raw); err != nil {
		return slidejson.SlideJSON{}, err
	}

	if p.Title != nil || p.Layout != nil {
		if err := svc.store.UpdateSlideMeta(ctx, slideID, cur.Title, cur.Layout); err != nil {
			return slidejson.SlideJSON{}, err
		}
	}
	if _, herr := sb.Read(model.SlideHTMLPath(slideID)); herr == nil {
		if err := svc.store.SetOutlineDirty(ctx, slideID, true); err != nil {
			return slidejson.SlideJSON{}, err
		}
	}
	return cur, nil
}
