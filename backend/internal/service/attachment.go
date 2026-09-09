package service

import (
	"context"
	"errors"
	"io"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// AttachmentService is the project-authorized gateway to the on-disk image store.
type AttachmentService struct {
	store store.Store
}

func NewAttachmentService(s store.Store) *AttachmentService { return &AttachmentService{store: s} }

func (svc *AttachmentService) Upload(ctx context.Context, projectID, originalName string, source io.Reader) (attachment.Meta, error) {
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return attachment.Meta{}, err
	}
	meta, err := attachment.Create(ctx, project.WorkDir, project.ID, model.MustShortID("att"), originalName, source)
	if err != nil {
		return attachment.Meta{}, attachmentAgentError(err, "upload_attachment")
	}
	return meta, nil
}

func (svc *AttachmentService) Get(ctx context.Context, projectID, attachmentID string) (attachment.Meta, error) {
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return attachment.Meta{}, err
	}
	meta, err := attachment.Load(project.WorkDir, attachmentID)
	if err != nil || meta.ProjectID != project.ID {
		if err == nil {
			err = &attachment.Error{Code: attachment.NotFound}
		}
		return attachment.Meta{}, attachmentAgentError(err, "get_attachment")
	}
	return meta, nil
}

func (svc *AttachmentService) Content(ctx context.Context, projectID, attachmentID, variant string) (attachment.Meta, []byte, error) {
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return attachment.Meta{}, nil, err
	}
	meta, raw, err := attachment.Read(ctx, project.WorkDir, project.ID, attachmentID, variant)
	if err != nil {
		return attachment.Meta{}, nil, attachmentAgentError(err, "read_attachment")
	}
	return meta, raw, nil
}

// ResolveReferences verifies that every client-provided ID is a current image
// in the project, and captures stable message metadata before a Run starts.
func (svc *AttachmentService) ResolveReferences(ctx context.Context, project model.Project, ids []string) ([]model.AttachmentReference, error) {
	if len(ids) > model.MaxRunAttachments {
		return nil, model.NewAgentError("ATTACHMENT_LIMIT_EXCEEDED", "resolve_attachments", nil)
	}
	seen := make(map[string]bool, len(ids))
	refs := make([]model.AttachmentReference, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			return nil, model.NewAgentError("ATTACHMENT_INVALID", "resolve_attachments", errors.New("duplicate attachment id"))
		}
		seen[id] = true
		meta, _, err := attachment.Read(ctx, project.WorkDir, project.ID, id, "original")
		if err != nil {
			return nil, attachmentAgentError(err, "resolve_attachments")
		}
		refs = append(refs, meta.Reference())
	}
	return refs, nil
}

func attachmentAgentError(err error, operation string) error {
	var attachmentErr *attachment.Error
	if errors.As(err, &attachmentErr) {
		return model.NewAgentError(string(attachmentErr.Code), operation, err)
	}
	return err
}
