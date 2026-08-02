package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type SpecHandler struct {
	svc *service.SpecService
}

func NewSpecHandler(svc *service.SpecService) *SpecHandler {
	return &SpecHandler{svc: svc}
}

func (h *SpecHandler) GetProject(c *gin.Context) {
	view, err := h.svc.EnsureProject(c.Request.Context(), c.Param("id"))
	if err != nil {
		AbortWithError(c, specError(err))
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *SpecHandler) PatchProject(c *gin.Context) {
	var body struct {
		ExpectedRevision int          `json:"expected_revision"`
		Outline          spec.Outline `json:"outline"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	outline, err := h.svc.ReplaceOutline(c.Request.Context(), c.Param("id"), body.ExpectedRevision, body.Outline)
	if err != nil {
		AbortWithError(c, specError(err))
		return
	}
	c.JSON(http.StatusOK, outline)
}

func (h *SpecHandler) PatchDesign(c *gin.Context) {
	var body struct {
		ExpectedRevision int         `json:"expected_revision"`
		Design           spec.Design `json:"design"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	design, err := h.svc.ReplaceDesign(c.Request.Context(), c.Param("id"), body.ExpectedRevision, body.Design)
	if err != nil {
		AbortWithError(c, specError(err))
		return
	}
	c.JSON(http.StatusOK, design)
}

func (h *SpecHandler) GetSlide(c *gin.Context) {
	slide, state, err := h.svc.GetSlide(c.Request.Context(), c.Param("id"))
	if err != nil {
		AbortWithError(c, specError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"spec": slide, "materialization": state})
}

func (h *SpecHandler) PatchSlide(c *gin.Context) {
	var body struct {
		ExpectedRevision int            `json:"expected_revision"`
		Spec             spec.SlideSpec `json:"spec"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	slide, err := h.svc.PatchSlide(c.Request.Context(), c.Param("id"), body.ExpectedRevision, body.Spec)
	if err != nil {
		AbortWithError(c, specError(err))
		return
	}
	c.JSON(http.StatusOK, slide)
}

func specError(err error) *APIError {
	switch {
	case errors.Is(err, service.ErrSpecRevisionConflict):
		return &APIError{HTTPStatus: http.StatusConflict, Code: "SPEC_REVISION_CONFLICT", Message: err.Error()}
	case errors.Is(err, spec.ErrReferenceBroken):
		return &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "SPEC_REFERENCE_BROKEN", Message: err.Error()}
	case errors.Is(err, spec.ErrInvalid):
		return &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "SPEC_INVALID", Message: err.Error()}
	case errors.Is(err, run.ErrRunNotFound):
		return ErrNotFound("spec resource not found")
	default:
		return ErrInternal(err.Error())
	}
}
