package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

type BlueprintHandler struct {
	svc *service.BlueprintService
}

func NewBlueprintHandler(svc *service.BlueprintService) *BlueprintHandler {
	return &BlueprintHandler{svc: svc}
}

func (h *BlueprintHandler) GetProject(c *gin.Context) {
	view, err := h.svc.EnsureProject(c.Request.Context(), c.Param("id"))
	if err != nil {
		AbortWithError(c, blueprintError(err))
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *BlueprintHandler) PatchProject(c *gin.Context) {
	var body struct {
		ExpectedRevision int            `json:"expected_revision"`
		Deck             blueprint.Deck `json:"deck"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	deck, err := h.svc.ReplaceDeck(c.Request.Context(), c.Param("id"), body.ExpectedRevision, body.Deck)
	if err != nil {
		AbortWithError(c, blueprintError(err))
		return
	}
	c.JSON(http.StatusOK, deck)
}

func (h *BlueprintHandler) PatchDesignSpec(c *gin.Context) {
	var body struct {
		ExpectedRevision int                  `json:"expected_revision"`
		DesignSpec       blueprint.DesignSpec `json:"design_spec"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	spec, err := h.svc.ReplaceDesignSpec(c.Request.Context(), c.Param("id"), body.ExpectedRevision, body.DesignSpec)
	if err != nil {
		AbortWithError(c, blueprintError(err))
		return
	}
	c.JSON(http.StatusOK, spec)
}

func (h *BlueprintHandler) GetSlide(c *gin.Context) {
	slide, state, err := h.svc.GetSlide(c.Request.Context(), c.Param("id"))
	if err != nil {
		AbortWithError(c, blueprintError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"blueprint": slide, "materialization": state})
}

func (h *BlueprintHandler) PatchSlide(c *gin.Context) {
	var body struct {
		ExpectedRevision int             `json:"expected_revision"`
		Blueprint        blueprint.Slide `json:"blueprint"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	slide, err := h.svc.PatchSlide(c.Request.Context(), c.Param("id"), body.ExpectedRevision, body.Blueprint)
	if err != nil {
		AbortWithError(c, blueprintError(err))
		return
	}
	c.JSON(http.StatusOK, slide)
}

func blueprintError(err error) *APIError {
	switch {
	case errors.Is(err, service.ErrBlueprintRevisionConflict):
		return &APIError{HTTPStatus: http.StatusConflict, Code: "BLUEPRINT_REVISION_CONFLICT", Message: err.Error()}
	case errors.Is(err, blueprint.ErrReferenceBroken):
		return &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "BLUEPRINT_REFERENCE_BROKEN", Message: err.Error()}
	case errors.Is(err, blueprint.ErrInvalid):
		return &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "BLUEPRINT_INVALID", Message: err.Error()}
	case errors.Is(err, run.ErrRunNotFound):
		return ErrNotFound("blueprint resource not found")
	default:
		return ErrInternal(err.Error())
	}
}
