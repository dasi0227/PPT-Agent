package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ErrSlideHTMLMissing 表示 slide 记录存在但磁盘上 index.html 尚未产出。
var ErrSlideHTMLMissing = errors.New("service: slide html not rendered")

// ReadHTML 读某页 index.html 原始字节，供 iframe 预览端点透传。
// slide 元数据缺失 → gorm.ErrRecordNotFound；产物文件缺失 → ErrSlideHTMLMissing；
// 越界路径由 sandbox 拦截并返回错误。
func (svc *SlideService) ReadHTML(ctx context.Context, slideID string) ([]byte, error) {
	sl, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return nil, err
	}
	proj, err := svc.store.GetProject(ctx, sl.ProjectID)
	if err != nil {
		return nil, err
	}
	sb, err := tools.NewSandbox(proj.WorkDir)
	if err != nil {
		return nil, err
	}
	if sl.HTMLPath == "" {
		return nil, ErrSlideHTMLMissing
	}
	raw, err := sb.Read(sl.HTMLPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSlideHTMLMissing
		}
		return nil, err
	}
	return raw, nil
}

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
//
// 手动路径：project 有活跃 run 时互斥（ErrRunActive）。
func (svc *SlideService) PatchContent(ctx context.Context, slideID string, p SlidePatch) (slidejson.SlideJSON, error) {
	sl, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	// 并发护栏：project 有活跃 run 时，手动写与 AI 写互斥（RUN_ACTIVE）。
	if active, err := svc.store.HasActiveRun(ctx, sl.ProjectID); err != nil {
		return slidejson.SlideJSON{}, err
	} else if active {
		return slidejson.SlideJSON{}, ErrRunActive
	}
	return svc.patchContentCore(ctx, sl, p)
}

// patchContentCore 是 PatchContent 的无互斥核心（供 AI 大纲编辑 runner 复用；AI run 本身即活跃 run）。
func (svc *SlideService) patchContentCore(ctx context.Context, sl model.Slide, p SlidePatch) (slidejson.SlideJSON, error) {
	slideID := sl.ID
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

// AddSlide 在锚点页后插入一张空白页：写空白 slide.json + 落库（order 取锚点与后继中值）。
// 受 RUN_ACTIVE 互斥；不产版本快照。afterSlideID 为空时追加到末尾。
func (svc *SlideService) AddSlide(ctx context.Context, projectID, afterSlideID, layout string) (model.Slide, error) {
	if active, err := svc.store.HasActiveRun(ctx, projectID); err != nil {
		return model.Slide{}, err
	} else if active {
		return model.Slide{}, ErrRunActive
	}
	return svc.addSlideCore(ctx, projectID, afterSlideID, layout)
}

// addSlideCore 是 AddSlide 的无互斥核心（供 AI 大纲编辑 runner 复用）。
func (svc *SlideService) addSlideCore(ctx context.Context, projectID, afterSlideID, layout string) (model.Slide, error) {
	if layout == "" || !slidejson.IsValidLayout(layout) {
		layout = "bullets"
	}
	proj, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return model.Slide{}, err
	}
	sb, err := tools.NewSandbox(proj.WorkDir)
	if err != nil {
		return model.Slide{}, err
	}
	slides, err := svc.store.ListSlides(ctx, projectID)
	if err != nil {
		return model.Slide{}, err
	}

	newOrder := svc.insertOrder(slides, afterSlideID)
	newIdx := len(slides)
	id := svc.newID()
	doc := slidejson.SlideJSON{ID: id, Idx: newIdx, Layout: layout, Title: ""}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return model.Slide{}, err
	}
	if err := sb.Write(model.SlideJSONPath(id), raw); err != nil {
		return model.Slide{}, err
	}
	sl := model.Slide{
		ID: id, ProjectID: projectID, Idx: newIdx, Layout: layout, Title: "",
		JSONPath: model.SlideJSONPath(id), HTMLPath: model.SlideHTMLPath(id), Order: newOrder,
	}
	if err := svc.store.InsertSlide(ctx, sl); err != nil {
		return model.Slide{}, err
	}
	return sl, nil
}

// insertOrder 计算锚点页之后的插入 order：与后继页求中值；无后继则锚点 +10；锚点缺失则追加到末尾。
func (svc *SlideService) insertOrder(slides []model.Slide, afterSlideID string) int {
	if len(slides) == 0 {
		return 0
	}
	anchor := -1
	for i, s := range slides {
		if s.ID == afterSlideID {
			anchor = i
			break
		}
	}
	if anchor < 0 {
		return slides[len(slides)-1].Order + 10
	}
	if anchor == len(slides)-1 {
		return slides[anchor].Order + 10
	}
	return (slides[anchor].Order + slides[anchor+1].Order) / 2
}

// DeleteSlide 删除单页：删 DB 行 + slides/<id>/ 目录 + 该页 versions 目录（无回收站）。
// 受 RUN_ACTIVE 互斥；不产版本快照。
func (svc *SlideService) DeleteSlide(ctx context.Context, slideID string) error {
	sl, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return err
	}
	if active, err := svc.store.HasActiveRun(ctx, sl.ProjectID); err != nil {
		return err
	} else if active {
		return ErrRunActive
	}
	return svc.deleteSlideCore(ctx, sl)
}

// deleteSlideCore 是 DeleteSlide 的无互斥核心（供 AI 大纲编辑 runner 复用）。
func (svc *SlideService) deleteSlideCore(ctx context.Context, sl model.Slide) error {
	proj, err := svc.store.GetProject(ctx, sl.ProjectID)
	if err != nil {
		return err
	}
	if err := svc.store.DeleteSlideByID(ctx, sl.ID); err != nil {
		return err
	}
	// best-effort 清磁盘：整页目录与版本目录一并移除（无回收站）。
	_ = os.RemoveAll(filepath.Join(proj.WorkDir, filepath.FromSlash(model.SlideDir(sl.ID))))
	_ = os.RemoveAll(filepath.Join(proj.WorkDir, filepath.FromSlash("versions/slide-"+sl.ID)))
	return nil
}

// ReorderSlides 按给定顺序重排某 project 的页（order=i*10，磁盘零迁移）。
// 受 RUN_ACTIVE 互斥；不产版本快照。
func (svc *SlideService) ReorderSlides(ctx context.Context, projectID string, orderedIDs []string) error {
	if active, err := svc.store.HasActiveRun(ctx, projectID); err != nil {
		return err
	} else if active {
		return ErrRunActive
	}
	return svc.reorderSlidesCore(ctx, projectID, orderedIDs)
}

// reorderSlidesCore 是 ReorderSlides 的无互斥核心（供 AI 大纲编辑 runner 复用）。
func (svc *SlideService) reorderSlidesCore(ctx context.Context, projectID string, orderedIDs []string) error {
	orderByID := make(map[string]int, len(orderedIDs))
	for i, id := range orderedIDs {
		orderByID[id] = i * 10
	}
	return svc.store.SetSlidesOrder(ctx, projectID, orderByID)
}
