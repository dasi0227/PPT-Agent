package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type PromptHandler struct {
	svc *service.PromptService
}

func NewPromptHandler(svc *service.PromptService) *PromptHandler {
	return &PromptHandler{svc: svc}
}

type promptWriteRequest struct {
	Name  string            `json:"name"`
	Desc  string            `json:"desc"`
	Value string            `json:"value"`
	Tags  []model.PromptTag `json:"tags"`
}

func (h *PromptHandler) List(c *gin.Context) {
	prompts, err := h.svc.List(c.Request.Context())
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"prompts": prompts})
}

func (h *PromptHandler) Get(c *gin.Context) {
	prompt, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, prompt)
}

func (h *PromptHandler) Create(c *gin.Context) {
	request, ok := bindPromptRequest(c)
	if !ok {
		return
	}
	prompt, err := h.svc.Create(c.Request.Context(), request.params())
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, prompt)
}

func (h *PromptHandler) Update(c *gin.Context) {
	request, ok := bindPromptRequest(c)
	if !ok {
		return
	}
	prompt, err := h.svc.Update(c.Request.Context(), c.Param("id"), request.params())
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, prompt)
}

func (h *PromptHandler) Patch(c *gin.Context) {
	var request struct {
		Disabled *bool `json:"disabled"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || request.Disabled == nil {
		AbortWithError(c, &APIError{
			HTTPStatus: http.StatusBadRequest, Code: "PROMPT_INVALID",
			Message: "disabled 字段不能为空", Details: map[string]any{"field": "disabled"},
		})
		return
	}
	prompt, err := h.svc.SetDisabled(c.Request.Context(), c.Param("id"), *request.Disabled)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, prompt)
}

func (h *PromptHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func bindPromptRequest(c *gin.Context) (promptWriteRequest, bool) {
	var request promptWriteRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		AbortWithError(c, &APIError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "PROMPT_INVALID",
			Message:    "请求内容格式无效",
			Details:    map[string]any{"field": "body"},
		})
		return promptWriteRequest{}, false
	}
	return request, true
}

func (r promptWriteRequest) params() service.PromptWriteParams {
	return service.PromptWriteParams{Name: r.Name, Desc: r.Desc, Value: r.Value, Tags: r.Tags}
}

func (h *PromptHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPromptNotFound):
		AbortWithError(c, &APIError{
			HTTPStatus: http.StatusNotFound, Code: "PROMPT_NOT_FOUND",
			Message: "提示词不存在", Details: map[string]any{},
		})
	case errors.Is(err, service.ErrPromptNameConflict):
		var conflict *service.PromptNameConflictError
		errors.As(err, &conflict)
		field := "name"
		if conflict != nil {
			field = conflict.Field
		}
		AbortWithError(c, &APIError{
			HTTPStatus: http.StatusConflict, Code: "PROMPT_NAME_CONFLICT",
			Message: "提示词名称已存在", Details: map[string]any{"field": field},
		})
	case errors.Is(err, service.ErrPromptInvalid):
		var invalid *service.PromptValidationError
		errors.As(err, &invalid)
		field := "prompt"
		message := "提示词内容无效"
		if invalid != nil {
			field, message = invalid.Field, invalid.Message
		}
		AbortWithError(c, &APIError{
			HTTPStatus: http.StatusBadRequest, Code: "PROMPT_INVALID",
			Message: message, Details: map[string]any{"field": field},
		})
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}
