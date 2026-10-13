package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/branding"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *ProjectHandler) Decorations(c *gin.Context) {
	project, err := h.svc.GetProject(c.Request.Context(), c.Param("id"))
	if err != nil {
		handleAppearanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, project.Decorations)
}

func (h *ProjectHandler) SetDecorations(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var updates map[string]string
	decoder := json.NewDecoder(c.Request.Body)
	if decoder.Decode(&updates) != nil || decoder.Decode(new(any)) != io.EOF {
		AbortWithError(c, ErrBadRequest("decoration positions must be a JSON object"))
		return
	}
	project, err := h.svc.SetDecorations(c.Request.Context(), c.Param("id"), updates)
	if err != nil {
		handleAppearanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectResponse(project))
}

func (h *ProjectHandler) UploadLogo(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, attachment.MaxFileBytes+1024*1024)
	file, header, err := c.Request.FormFile("file")
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		AbortWithError(c, ErrBadRequest("one logo image is required"))
		return
	}
	defer file.Close()
	if len(c.Request.MultipartForm.File) != 1 || len(c.Request.MultipartForm.File["file"]) != 1 {
		AbortWithError(c, ErrBadRequest("upload exactly one logo image"))
		return
	}
	project, err := h.svc.UploadLogo(c.Request.Context(), c.Param("id"), header.Filename, file)
	if err != nil {
		handleAppearanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectResponse(project))
}

func (h *ProjectHandler) Logo(c *gin.Context) {
	project, err := h.svc.GetProject(c.Request.Context(), c.Param("id"))
	if err != nil {
		handleAppearanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, project.Logo)
}

func (h *ProjectHandler) LogoContent(c *gin.Context) {
	logo, raw, err := h.svc.LogoContent(c.Request.Context(), c.Param("id"))
	if err != nil {
		handleAppearanceError(c, err)
		return
	}
	if version := c.Query("v"); version != "" && version != logo.Hash {
		AbortWithError(c, ErrNotFound("logo image has changed"))
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("ETag", `"`+logo.Hash+`"`)
	c.Data(http.StatusOK, logo.MediaType, raw)
}

func (h *ProjectHandler) DeleteLogo(c *gin.Context) {
	project, err := h.svc.DeleteLogo(c.Request.Context(), c.Param("id"))
	if err != nil {
		handleAppearanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectResponse(project))
}

func handleAppearanceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project is currently locked"})
	case errors.Is(err, service.ErrGitCommitActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "GIT_COMMIT_ACTIVE", Message: "project has an active Git commit"})
	case errors.Is(err, service.ErrInvalidDecorations):
		AbortWithError(c, ErrBadRequest(err.Error()))
	case errors.Is(err, branding.ErrNotFound), errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project or logo not found"))
	default:
		var imageError *attachment.Error
		if errors.As(err, &imageError) {
			status := http.StatusBadRequest
			if imageError.Code == attachment.TooLarge {
				status = http.StatusRequestEntityTooLarge
			}
			AbortWithError(c, &APIError{HTTPStatus: status, Code: string(imageError.Code), Message: imageError.Error()})
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
	}
}
