package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// ErrVersionNotFound：回滚目标版本号不存在。
var ErrVersionNotFound = errors.New("service: target version not found")

// SlideService 提供单页读取与版本回滚（DATA-VERSION-004）。
type SlideService struct {
	store store.Store
	clock func() int64
	newID func() string
}

func NewSlideService(s store.Store) *SlideService {
	return &SlideService{store: s, clock: nowUnix, newID: func() string { return model.MustShortID("sli") }}
}

// GetSlide 按 id 返回单页元数据，并从文件投影 position/title/layout（文件为真相）。
func (svc *SlideService) GetSlide(ctx context.Context, id string) (model.Slide, error) {
	meta, err := svc.store.GetSlide(ctx, id)
	if err != nil {
		return model.Slide{}, err
	}
	siblings, err := svc.store.ListSlides(ctx, meta.ProjectID)
	if err != nil {
		return meta, nil
	}
	projected, err := projectSlidesFromFiles(ctx, svc.store, meta.ProjectID, siblings)
	if err != nil {
		return meta, nil
	}
	for _, slide := range projected {
		if slide.ID == id {
			return slide, nil
		}
	}
	return meta, nil
}

// ListVersions 返回某页的版本列表。
func (svc *SlideService) ListVersions(ctx context.Context, slideID string) ([]model.Version, error) {
	sl, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return nil, err
	}
	return svc.store.ListVersions(ctx, "slide_html", model.SlideHTMLVersionTarget(sl.ProjectID, sl.ID))
}

// RollbackSlide 回滚某页到 versionNo：用历史快照覆盖当前 html + 记一条**新版本**（DATA-VERSION-004）。
// 不删旧版本；slides.current_version 指向新版本。
func (svc *SlideService) RollbackSlide(ctx context.Context, slideID string, versionNo int) (model.Slide, error) {
	sl, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return model.Slide{}, err
	}
	proj, err := svc.store.GetProject(ctx, sl.ProjectID)
	if err != nil {
		return model.Slide{}, err
	}
	target := model.SlideHTMLVersionTarget(sl.ProjectID, sl.ID)

	// 定位历史版本快照路径。
	versions, err := svc.store.ListVersions(ctx, "slide_html", target)
	if err != nil {
		return model.Slide{}, err
	}
	var snapshotPath string
	for _, v := range versions {
		if v.VersionNo == versionNo {
			snapshotPath = v.SnapshotPath
			break
		}
	}
	if snapshotPath == "" {
		return model.Slide{}, ErrVersionNotFound
	}

	sandbox, err := artifactfs.NewSandbox(proj.WorkDir)
	if err != nil {
		return model.Slide{}, err
	}
	historical, err := sandbox.Read(snapshotPath)
	if err != nil {
		return model.Slide{}, fmt.Errorf("read snapshot %s: %w", snapshotPath, err)
	}
	previous, readErr := sandbox.Read(sl.HTMLPath)
	restoreCurrent := func() {
		if readErr == nil {
			_ = sandbox.Write(sl.HTMLPath, previous)
		} else {
			_ = sandbox.Delete(sl.HTMLPath)
		}
	}

	// 1) 用历史内容覆盖当前 html。
	if err := sandbox.Write(sl.HTMLPath, historical); err != nil {
		return model.Slide{}, err
	}

	// 2) 记一条新版本（内容=历史，version_no 递增），关联无 run。
	newNo, err := svc.store.NextVersionNo(ctx, "slide_html", target)
	if err != nil {
		restoreCurrent()
		return model.Slide{}, err
	}
	snap := model.SlideHTMLVersionSnapshot(sl.ID, newNo)
	if err := sandbox.Write(snap, historical); err != nil {
		restoreCurrent()
		return model.Slide{}, err
	}
	if err := svc.store.CreateVersion(ctx, model.Version{
		ID: svc.newID(), TargetType: "slide_html", TargetID: target, VersionNo: newNo,
		SnapshotPath: snap, CreatedAt: svc.clock(),
	}); err != nil {
		restoreCurrent()
		_ = sandbox.Delete(snap)
		return model.Slide{}, err
	}

	// 3) current_version 指向新版本（DATA-VERSION-005）。
	if err := svc.store.SetSlideVersion(ctx, sl.ID, newNo); err != nil {
		restoreCurrent()
		_ = svc.store.DeleteVersion(ctx, "slide_html", target, newNo)
		_ = sandbox.Delete(snap)
		return model.Slide{}, err
	}

	sl.CurrentVersion = newNo
	return sl, nil
}

func nowUnix() int64 { return time.Now().Unix() }
