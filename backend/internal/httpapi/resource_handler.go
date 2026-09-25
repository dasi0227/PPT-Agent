package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/gin-gonic/gin"
)

type ResourceHandler struct{ svc *service.ResourceService }

func NewResourceHandler(svc *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

func resourceBody(c *gin.Context, v any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		AbortWithError(c, ErrBadRequest("请求格式无效"))
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		AbortWithError(c, ErrBadRequest("请求必须是一个 JSON 对象"))
		return false
	}
	return true
}

func resourceFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrResourceNotFound):
		AbortWithError(c, ErrNotFound("资源不存在"))
	case errors.Is(err, store.ErrResourceConflict):
		AbortWithError(c, ErrConflict("资源 ID 或短语名称已存在"))
	case errors.Is(err, service.ErrInvalidRepositoryID), errors.Is(err, store.ErrTagNotFound):
		AbortWithError(c, ErrBadRequest(err.Error()))
	case errors.Is(err, service.ErrRepositoryCorrupt), errors.Is(err, service.ErrRepositoryFileTooLarge), errors.Is(err, service.ErrUnsafeRepositoryPath), errors.Is(err, os.ErrNotExist):
		AbortWithError(c, &APIError{HTTPStatus: 422, Code: "RESOURCE_INVALID", Message: err.Error()})
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ResourceHandler) Register(c *gin.Context) {
	var req struct {
		Type        string   `json:"type"`
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		Disabled    bool     `json:"disabled"`
	}
	if !resourceBody(c, &req) {
		return
	}
	r, err := h.svc.Register(c.Request.Context(), model.Resource{Type: req.Type, ID: req.ID, Name: req.Name, Description: req.Description, Tags: req.Tags, Disabled: req.Disabled})
	if err != nil {
		resourceFailure(c, err)
		return
	}
	c.JSON(201, r)
}

func (h *ResourceHandler) Patch(c *gin.Context) {
	var p service.ResourcePatch
	if !resourceBody(c, &p) {
		return
	}
	r, err := h.svc.Patch(c.Request.Context(), c.Param("type"), c.Param("id"), p)
	if err != nil {
		resourceFailure(c, err)
		return
	}
	c.JSON(200, r)
}

func (h *ResourceHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("type"), c.Param("id")); err != nil {
		resourceFailure(c, err)
		return
	}
	c.Status(204)
}

func (h *ResourceHandler) ListSnippets(c *gin.Context) {
	v, err := h.svc.Snippets(c.Request.Context())
	if err != nil {
		resourceFailure(c, err)
		return
	}
	c.JSON(200, gin.H{"snippets": v})
}

func (h *ResourceHandler) GetSnippet(c *gin.Context) {
	v, err := h.svc.Snippet(c.Request.Context(), c.Param("id"))
	if err != nil {
		resourceFailure(c, err)
		return
	}
	c.JSON(200, v)
}

func (h *ResourceHandler) CreateSnippet(c *gin.Context) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Content     string   `json:"content"`
		Tags        []string `json:"tags"`
		Disabled    bool     `json:"disabled"`
	}
	if !resourceBody(c, &req) {
		return
	}
	v, err := h.svc.CreateSnippet(c.Request.Context(), model.Resource{Name: req.Name, Description: req.Description, Tags: req.Tags, Disabled: req.Disabled}, req.Content)
	if err != nil {
		resourceFailure(c, err)
		return
	}
	c.JSON(201, v)
}

func (h *ResourceHandler) WriteSnippet(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if !resourceBody(c, &req) {
		return
	}
	v, err := h.svc.WriteSnippet(c.Request.Context(), c.Param("id"), req.Content)
	if err != nil {
		resourceFailure(c, err)
		return
	}
	c.JSON(200, v)
}

func (h *ResourceHandler) ListTags(c *gin.Context) {
	tags, err := h.svc.Tags(c.Request.Context(), c.Query("scope"))
	if err != nil {
		resourceFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}
