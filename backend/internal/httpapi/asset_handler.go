package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

type AssetHandler struct {
	svc *service.AssetService
}

func NewAssetHandler(svc *service.AssetService) *AssetHandler {
	return &AssetHandler{svc: svc}
}

type assetResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Version      string   `json:"version"`
	Source       string   `json:"source"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags,omitempty"`
	ManifestPath string   `json:"manifest_path"`
	Dir          string   `json:"dir"`
	CreatedAt    int64    `json:"created_at"`
	UpdatedAt    int64    `json:"updated_at"`
}

type createAssetRequest struct {
	Manifest asset.Manifest    `json:"manifest"`
	Payload  map[string]string `json:"payload"`
}

type patchAssetRequest struct {
	Edits []patchAssetEditRequest `json:"edits"`
}

type rollbackAssetRequest struct {
	VersionNo *int `json:"version_no"`
}

type patchAssetEditRequest struct {
	File    string `json:"file"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func (h *AssetHandler) List(c *gin.Context) {
	kind := c.Query("kind")
	if kind != "" && !validAssetKind(kind) {
		AbortWithError(c, ErrBadRequest("kind must be one of layout, component, theme, fx"))
		return
	}
	assets, err := h.svc.ListAssets(c.Request.Context(), kind)
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	out := make([]assetResponse, len(assets))
	for i, a := range assets {
		out[i] = toAssetResponse(a)
	}
	c.JSON(http.StatusOK, out)
}

func (h *AssetHandler) Get(c *gin.Context) {
	a, err := h.svc.GetAsset(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			AbortWithError(c, ErrNotFound("asset not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, toAssetResponse(a))
}

func (h *AssetHandler) Create(c *gin.Context) {
	var req createAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	a, err := h.svc.CreateAsset(c.Request.Context(), service.CreateAssetParams{
		Manifest: req.Manifest,
		Payload:  req.Payload,
	})
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, toAssetResponse(a))
	case service.IsValidationError(err):
		AbortWithError(c, ErrValidationFailed(err.Error()))
	case errors.Is(err, service.ErrAssetConflict):
		AbortWithError(c, ErrConflict("asset already exists"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *AssetHandler) Patch(c *gin.Context) {
	var req patchAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	edits := make([]service.AssetPatchEdit, len(req.Edits))
	for i, e := range req.Edits {
		edits[i] = service.AssetPatchEdit{File: e.File, OldText: e.OldText, NewText: e.NewText}
	}
	a, err := h.svc.PatchAsset(c.Request.Context(), c.Param("id"), service.PatchAssetParams{Edits: edits})
	switch {
	case err == nil:
		c.JSON(http.StatusOK, toAssetResponse(a))
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("asset not found"))
	case service.IsValidationError(err):
		AbortWithError(c, ErrValidationFailed(err.Error()))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *AssetHandler) Delete(c *gin.Context) {
	err := h.svc.DeleteAsset(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("asset not found"))
	case service.IsPresetDeleteError(err):
		AbortWithError(c, ErrConflict("preset asset cannot be deleted"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *AssetHandler) Rollback(c *gin.Context) {
	var req rollbackAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.VersionNo == nil {
		AbortWithError(c, ErrBadRequest("version_no is required"))
		return
	}
	a, err := h.svc.RollbackAsset(c.Request.Context(), c.Param("id"), *req.VersionNo)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, toAssetResponse(a))
	case errors.Is(err, service.ErrAssetVersionMiss):
		AbortWithError(c, ErrNotFound("target version not found"))
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("asset not found"))
	case service.IsValidationError(err):
		AbortWithError(c, ErrValidationFailed(err.Error()))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func toAssetResponse(a model.Asset) assetResponse {
	return assetResponse{
		ID: a.ID, Name: a.Name, Kind: a.Kind, Version: a.Version, Source: a.Source,
		Description: a.Description, Tags: a.Tags, ManifestPath: a.ManifestPath, Dir: a.Dir,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func validAssetKind(kind string) bool {
	switch kind {
	case "layout", "component", "theme", "fx":
		return true
	default:
		return false
	}
}
