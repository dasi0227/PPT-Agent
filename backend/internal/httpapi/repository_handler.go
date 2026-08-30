package httpapi

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/seed"
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
	raw, err := fs.ReadFile(seed.FS(), "common/base.css")
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/css; charset=utf-8", raw)
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

func (h *RepositoryHandler) PatchSkill(c *gin.Context) {
	var request struct {
		Disabled *bool `json:"disabled"`
	}
	if c.ShouldBindJSON(&request) != nil || request.Disabled == nil {
		AbortWithError(c, ErrBadRequest("disabled is required"))
		return
	}
	value, err := h.skills.SetDisabled(c.Param("id"), *request.Disabled)
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
