package httpapi

import (
	"errors"
	"net/http"

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
		}
		api := &APIError{HTTPStatus: status, Code: sourceErr.Code, Message: sourceErr.Message}

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
