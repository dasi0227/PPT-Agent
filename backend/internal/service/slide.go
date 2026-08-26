package service

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	htmlPath := model.SlideHTMLPath(sl.ID)
	previous, readErr := sandbox.Read(htmlPath)
	materializationPath := model.SlideMaterializationPath(sl.ID)
	previousMaterialization, materializationReadErr := sandbox.Read(materializationPath)
	restoreCurrent := func() {
		if readErr == nil {
			_ = sandbox.Write(htmlPath, previous)
		} else {
			_ = sandbox.Delete(htmlPath)
		}
		if materializationReadErr == nil {
			_ = sandbox.Write(materializationPath, previousMaterialization)
		}
	}

	// 1) 用历史内容覆盖当前 html。
	if err := sandbox.Write(htmlPath, historical); err != nil {
		return model.Slide{}, err
	}
	if err := sandbox.Delete(materializationPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		restoreCurrent()
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
