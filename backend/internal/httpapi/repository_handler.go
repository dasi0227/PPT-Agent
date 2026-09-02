package httpapi

import (
	"errors"
	"net/http"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type RepositoryHandler struct {
	themes     *service.ThemeService
	components *service.ComponentService
	skills     *service.SkillService
}

func NewRepositoryHandler(themes *service.ThemeService, components *service.ComponentService, skills *service.SkillService) *RepositoryHandler {
	return &RepositoryHandler{themes: themes, components: components, skills: skills}
}

func (h *RepositoryHandler) RuntimeBaseCSS(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/css; charset=utf-8", runtimeassets.BaseCSS())
}

func (h *RepositoryHandler) ListThemes(c *gin.Context) {
	values, err := h.themes.List()
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"themes": values})
}

func (h *RepositoryHandler) GetTheme(c *gin.Context) {
	value, err := h.themes.Get(c.Param("id"))
	if err != nil {
		h.repositoryError(c, err, "theme not found")
		return
	}
	c.JSON(http.StatusOK, value)
}

func (h *RepositoryHandler) PatchTheme(c *gin.Context) {
	var request struct {
		Name        *string           `json:"name"`
		Description *string           `json:"description"`
		Tags        *[]model.ThemeTag `json:"tags"`
	}
	if c.ShouldBindJSON(&request) != nil || request.Name == nil || request.Description == nil || request.Tags == nil {
		AbortWithError(c, ErrBadRequest("name, description, and tags are required"))
		return
	}
	value, err := h.themes.UpdateMetadata(c.Param("id"), *request.Name, *request.Description, *request.Tags)
	if err != nil {
		h.repositoryError(c, err, "theme not found")
		return
	}
	c.JSON(http.StatusOK, value)
}

func (h *RepositoryHandler) DeleteTheme(c *gin.Context) {
	if err := h.themes.Delete(c.Param("id")); err != nil {
		h.repositoryError(c, err, "theme not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *RepositoryHandler) ThemeCSS(c *gin.Context) {
	raw, err := h.themes.CSS(c.Param("id"))
	if err != nil {
		h.repositoryError(c, err, "theme not found")
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/css; charset=utf-8", raw)
}

func (h *RepositoryHandler) ListComponents(c *gin.Context) {
	values, err := h.components.List()
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"components": values})
}

func (h *RepositoryHandler) GetComponent(c *gin.Context) {
	value, err := h.components.Get(c.Param("id"))
	if err != nil {
		h.repositoryError(c, err, "component not found")
		return
	}
	c.JSON(http.StatusOK, value)
}

func (h *RepositoryHandler) PatchComponent(c *gin.Context) {
	var request struct {
		Disabled    *bool                 `json:"disabled"`
		Name        *string               `json:"name"`
		Description *string               `json:"description"`
		Tags        *[]model.ComponentTag `json:"tags"`
	}
	if c.ShouldBindJSON(&request) != nil {
		AbortWithError(c, ErrBadRequest("request body is invalid"))
		return
	}
	metadataUpdate := request.Name != nil || request.Description != nil || request.Tags != nil
	if request.Disabled == nil && !metadataUpdate {
		AbortWithError(c, ErrBadRequest("disabled or metadata is required"))
		return
	}
	if metadataUpdate && (request.Name == nil || request.Description == nil || request.Tags == nil) {
		AbortWithError(c, ErrBadRequest("name, description, and tags are required"))
		return
	}
	value, err := h.components.Get(c.Param("id"))
	if err == nil && metadataUpdate {
		value, err = h.components.UpdateMetadata(c.Param("id"), *request.Name, *request.Description, *request.Tags)
	}
	if err == nil && request.Disabled != nil {
		value, err = h.components.SetDisabled(c.Param("id"), *request.Disabled)
	}
	if err != nil {
		h.repositoryError(c, err, "component not found")
		return
	}
	c.JSON(http.StatusOK, value)
}

func (h *RepositoryHandler) DeleteComponent(c *gin.Context) {
	if err := h.components.Delete(c.Param("id")); err != nil {
		h.repositoryError(c, err, "component not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *RepositoryHandler) ListSkills(c *gin.Context) {
	values, err := h.skills.List()
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": values})
}

func (h *RepositoryHandler) GetSkill(c *gin.Context) {
	value, err := h.skills.Get(c.Param("id"))
	if err != nil {
		h.repositoryError(c, err, "skill not found")
		return
	}
	c.JSON(http.StatusOK, value)
}

func (h *RepositoryHandler) DeleteSkill(c *gin.Context) {
	if err := h.skills.Delete(c.Param("id")); err != nil {
		h.repositoryError(c, err, "skill not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *RepositoryHandler) PatchSkill(c *gin.Context) {
	var request struct {
		Disabled    *bool             `json:"disabled"`
		Name        *string           `json:"name"`
		Description *string           `json:"description"`
		Tags        *[]model.SkillTag `json:"tags"`
	}
	if c.ShouldBindJSON(&request) != nil {
		AbortWithError(c, ErrBadRequest("request body is invalid"))
		return
	}
	metadataUpdate := request.Name != nil || request.Description != nil || request.Tags != nil
	if request.Disabled == nil && !metadataUpdate {
		AbortWithError(c, ErrBadRequest("disabled or metadata is required"))
		return
	}
	if metadataUpdate && (request.Name == nil || request.Description == nil || request.Tags == nil) {
		AbortWithError(c, ErrBadRequest("name, description, and tags are required"))
		return
	}
	value, err := h.skills.Get(c.Param("id"))
	if err == nil && metadataUpdate {
		value, err = h.skills.UpdateMetadata(c.Param("id"), *request.Name, *request.Description, *request.Tags)
	}
	if err == nil && request.Disabled != nil {
		value, err = h.skills.SetDisabled(c.Param("id"), *request.Disabled)
	}
	if err != nil {
		h.repositoryError(c, err, "skill not found")
		return
	}
	c.JSON(http.StatusOK, value)
}

func (h *RepositoryHandler) repositoryError(c *gin.Context, err error, notFound string) {
	switch {
	case errors.Is(err, service.ErrInvalidRepositoryID):
		AbortWithError(c, ErrBadRequest("invalid repository id"))
	case errors.Is(err, service.ErrUnsafeRepositoryPath), errors.Is(err, service.ErrRepositoryFileTooLarge), errors.Is(err, service.ErrRepositoryCorrupt):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "REPOSITORY_INVALID", Message: err.Error()})
	default:
		AbortWithError(c, ErrNotFound(notFound))
	}
}
