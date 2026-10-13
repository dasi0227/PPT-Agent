package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/branding"
	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
)

var ErrInvalidDecorations = errors.New("invalid project decorations")

// Human settings use the same project writer boundary as theme selection.
func (svc *ProjectService) lockAppearance(ctx context.Context, id string) (model.Project, func(), error) {
	release := func() {}
	if svc.locks != nil {
		var err error
		release, err = svc.locks.Acquire(ctx, id, 5*time.Second)
		if err != nil {
			return model.Project{}, nil, ErrRunActive
		}
	}
	fail := func(err error) (model.Project, func(), error) { release(); return model.Project{}, nil, err }
	project, err := svc.store.GetProject(ctx, id)
	if err != nil {
		return fail(err)
	}
	if active, err := svc.store.HasActiveRun(ctx, id); err != nil {
		return fail(err)
	} else if active {
		return fail(ErrRunActive)
	}
	if active, err := svc.store.HasActiveGitCommit(ctx, id); err != nil {
		return fail(err)
	} else if active {
		return fail(ErrGitCommitActive)
	}
	unlockFiles := pptmutation.LockProject(project.WorkDir)
	return project, func() { unlockFiles(); release() }, nil
}

func (svc *ProjectService) SetDecorations(ctx context.Context, id string, updates map[string]string) (model.Project, error) {
	project, release, err := svc.lockAppearance(ctx, id)
	if err != nil {
		return model.Project{}, err
	}
	defer release()
	positions, err := project.Decorations.Merge(updates)
	if err != nil {
		return model.Project{}, fmt.Errorf("%w: %v", ErrInvalidDecorations, err)
	}
	updatedAt := svc.clock()
	if err := svc.store.UpdateProjectDecorations(ctx, id, positions, updatedAt); err != nil {
		return model.Project{}, err
	}
	project.Decorations, project.UpdatedAt = positions, updatedAt
	return project, nil
}

func (svc *ProjectService) UploadLogo(ctx context.Context, id, originalName string, source io.Reader) (model.Project, error) {
	logo, raw, err := branding.Verify(ctx, originalName, source)
	if err != nil {
		return model.Project{}, err
	}
	project, release, err := svc.lockAppearance(ctx, id)
	if err != nil {
		return model.Project{}, err
	}
	defer release()
	var previous []byte
	if project.Logo != nil {
		previous, err = branding.Read(project.WorkDir, project.Logo)
		if err != nil {
			return model.Project{}, err
		}
	}
	rollback := func() error {
		var removeErr error
		if project.Logo == nil || project.Logo.Extension != logo.Extension {
			removeErr = branding.Delete(project.WorkDir, logo)
		}
		if project.Logo != nil {
			return errors.Join(removeErr, branding.Write(project.WorkDir, project.Logo, previous))
		}
		return removeErr
	}
	if err = branding.Write(project.WorkDir, logo, raw); err != nil {
		return model.Project{}, errors.Join(err, rollback())
	}
	if project.Logo != nil && project.Logo.Extension != logo.Extension {
		if err = branding.Delete(project.WorkDir, project.Logo); err != nil {
			return model.Project{}, errors.Join(err, rollback())
		}
	}
	updatedAt := svc.clock()
	if err = svc.store.UpdateProjectLogo(ctx, id, logo, updatedAt); err != nil {
		return model.Project{}, errors.Join(err, rollback())
	}
	project.Logo, project.UpdatedAt = logo, updatedAt
	return project, nil
}

func (svc *ProjectService) DeleteLogo(ctx context.Context, id string) (model.Project, error) {
	project, release, err := svc.lockAppearance(ctx, id)
	if err != nil {
		return model.Project{}, err
	}
	defer release()
	if project.Logo == nil {
		return project, nil
	}
	previous, err := branding.Read(project.WorkDir, project.Logo)
	if err != nil {
		return model.Project{}, err
	}
	if err = branding.Delete(project.WorkDir, project.Logo); err != nil {
		return model.Project{}, errors.Join(err, branding.Write(project.WorkDir, project.Logo, previous))
	}
	updatedAt := svc.clock()
	if err = svc.store.UpdateProjectLogo(ctx, id, nil, updatedAt); err != nil {
		return model.Project{}, errors.Join(err, branding.Write(project.WorkDir, project.Logo, previous))
	}
	project.Logo, project.UpdatedAt = nil, updatedAt
	return project, nil
}

func (svc *ProjectService) LogoContent(ctx context.Context, id string) (*decoration.LogoMetadata, []byte, error) {
	project, err := svc.store.GetProject(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	release := pptmutation.ReadLockProject(project.WorkDir)
	defer release()
	// Read metadata after acquiring the same lock that protects file replacement.
	project, err = svc.store.GetProject(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	raw, err := branding.Read(project.WorkDir, project.Logo)
	return project.Logo, raw, err
}
