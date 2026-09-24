package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
)

func sourceAPIError(c *gin.Context, err error) {
	var sourceErr *service.SlideSourceError
	if errors.As(err, &sourceErr) {
		status := http.StatusConflict
		switch sourceErr.Code {
		case "SOURCE_REQUEST_INVALID":
			status = http.StatusBadRequest
		case "SOURCE_NOT_FOUND":
			status = http.StatusNotFound
		case "SOURCE_VALIDATION_FAILED":
			status = http.StatusUnprocessableEntity
		}
		api := &APIError{HTTPStatus: status, Code: sourceErr.Code, Message: sourceErr.Message}
		if sourceErr.Code == "HISTORY_CONFIRM_REQUIRED" {
			revision, _ := strconv.ParseInt(sourceErr.Pointer, 10, 64)
			api.Details = map[string]any{"project_id": c.Param("id"), "revision": revision}
		} else if sourceErr.Code == "SOURCE_VALIDATION_FAILED" {
			code := sourceErr.DiagnosticCode
			if code == "" {
				code = sourceErr.Code
			}
			diagnostic := gin.H{"code": code, "message": sourceErr.Message}
			if sourceErr.Pointer != "" {
				diagnostic["json_pointer"] = sourceErr.Pointer
			}
			if sourceErr.Line > 0 {
				diagnostic["line"] = sourceErr.Line
				diagnostic["column"] = sourceErr.Column
			}
			api.Details = map[string]any{"diagnostics": []gin.H{diagnostic}}
		}
		AbortWithError(c, api)
		return
	}
	if errors.Is(err, projecthistory.ErrConflict) || errors.Is(err, projecthistory.ErrBusy) {
		historyError(c, err)
		return
	}
	AbortWithError(c, ErrInternal(err.Error()))
}

func (r *Router) slideSourceGet(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	doc, err := r.source.Read(c.Request.Context(), c.Param("id"), c.Param("slide_id"), c.Query("kind"))
	if err != nil {
		sourceAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (r *Router) slideSourcePut(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var body struct {
		Content               *string `json:"content"`
		ExpectedSourceHash    string  `json:"expected_source_hash"`
		ExpectedSceneRevision *int64  `json:"expected_scene_revision"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20)
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == nil || body.ExpectedSourceHash == "" || body.ExpectedSceneRevision == nil {
		sourceAPIError(c, &service.SlideSourceError{Code: "SOURCE_REQUEST_INVALID", Message: "源文件保存参数无效"})
		return
	}
	discardRevision, _ := strconv.ParseInt(c.GetHeader("X-Discard-Future-Revision"), 10, 64)
	doc, changed, err := r.source.Save(c.Request.Context(), c.Param("id"), c.Param("slide_id"), c.Query("kind"), *body.Content, body.ExpectedSourceHash, *body.ExpectedSceneRevision, discardRevision)
	if err != nil {
		sourceAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"changed": changed, "document": doc})
}
