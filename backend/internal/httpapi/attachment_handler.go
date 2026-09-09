package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

type AttachmentHandler struct{ svc *service.AttachmentService }

func NewAttachmentHandler(svc *service.AttachmentService) *AttachmentHandler {
	return &AttachmentHandler{svc: svc}
}

type attachmentResponse struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	OriginalName string `json:"original_name"`
	MediaType    string `json:"media_type"`
	Extension    string `json:"extension"`
	SizeBytes    int64  `json:"size_bytes"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CreatedAt    int64  `json:"created_at"`
}

func toAttachmentResponse(meta attachment.Meta) attachmentResponse {
	return attachmentResponse{
		ID: meta.ID, ProjectID: meta.ProjectID, OriginalName: meta.OriginalName,
		MediaType: meta.MediaType, Extension: meta.Extension, SizeBytes: meta.SizeBytes,
		Width: meta.Width, Height: meta.Height, CreatedAt: meta.CreatedAt,
	}
}

func (h *AttachmentHandler) Upload(c *gin.Context) {
	if h == nil || h.svc == nil {
		AbortWithError(c, ErrInternal("attachment service unavailable"))
		return
	}
	// Multipart parsing happens before the attachment store can enforce its
	// byte limit. Leave room for MIME framing but reject oversized bodies early.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, attachment.MaxFileBytes+1024*1024)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		AbortWithError(c, ErrBadRequest("image file is required"))
		return
	}
	defer func() { _ = file.Close() }()
	meta, err := h.svc.Upload(c.Request.Context(), c.Param("id"), header.Filename, file)
	if err != nil {
		handleAttachmentError(c, err, "upload_attachment")
		return
	}
	c.JSON(http.StatusCreated, toAttachmentResponse(meta))
}

func (h *AttachmentHandler) Get(c *gin.Context) {
	meta, err := h.svc.Get(c.Request.Context(), c.Param("id"), c.Param("attachment_id"))
	if err != nil {
		handleAttachmentError(c, err, "get_attachment")
		return
	}
	c.JSON(http.StatusOK, toAttachmentResponse(meta))
}

func (h *AttachmentHandler) Content(c *gin.Context) {
	variant := strings.TrimSpace(c.DefaultQuery("variant", "thumbnail"))
	if variant != "thumbnail" && variant != "original" {
		AbortWithError(c, ProjectAgentError(model.NewAgentError("ATTACHMENT_INVALID", "read_attachment", nil), "ATTACHMENT_INVALID", "read_attachment"))
		return
	}
	meta, raw, err := h.svc.Content(c.Request.Context(), c.Param("id"), c.Param("attachment_id"), variant)
	if err != nil {
		handleAttachmentError(c, err, "read_attachment")
		return
	}
	mediaType := meta.MediaType
	if variant == "thumbnail" {
		mediaType = "image/webp"
	}
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, mediaType, raw)
}

func handleAttachmentError(c *gin.Context, err error, operation string) {
	var agentErr *model.AgentError
	if errors.As(err, &agentErr) {
		AbortWithError(c, ProjectAgentError(agentErr, "INTERNAL", operation))
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, run.ErrRunNotFound) {
		AbortWithError(c, ErrNotFound("project not found"))
		return
	}
	AbortWithError(c, ErrInternal(err.Error()))
}
