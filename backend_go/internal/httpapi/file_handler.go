package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/fileopen"
	"github.com/gin-gonic/gin"
)

// Native application actions are only available through same-origin JSON requests
// to a local server. A link, image or cross-site form must never launch an app.
func localFileRequest(c *gin.Context) bool {
	host := c.Request.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	localHost := strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
	originOK := true
	if origin := c.GetHeader("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		originOK = err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host == c.Request.Host
	}
	media, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if !localHost || !originOK || c.GetHeader("Sec-Fetch-Site") == "cross-site" || media != "application/json" {
		AbortWithError(c, &APIError{HTTPStatus: 403, Code: "FILE_ACTION_FORBIDDEN", Message: "请从本机应用页面操作文件"})
		return false
	}
	return true
}
func fileActionError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "FILE_ACTION_FAILED"
	switch {
	case errors.Is(err, fileopen.ErrConflict), errors.Is(err, fileopen.ErrPickerBusy):
		status, code = 409, "FILE_SETTINGS_CONFLICT"
	case errors.Is(err, fileopen.ErrInvalid), errors.Is(err, fileopen.ErrApplication), errors.Is(err, fileopen.ErrPath):
		status, code = 400, "FILE_ACTION_INVALID"
	case errors.Is(err, fileopen.ErrNotFound):
		status, code = 404, "FILE_NOT_FOUND"
	case errors.Is(err, fileopen.ErrUnsupported):
		status, code = 422, "FILE_ACTION_UNSUPPORTED"
	}
	AbortWithError(c, &APIError{HTTPStatus: status, Code: code, Message: err.Error()})
}
func (r *Router) WithFileSettings(s *fileopen.Service) *Router {
	r.engine.GET("/api/v1/settings/files", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		value, err := s.Get(c.Request.Context())
		if err != nil {
			fileActionError(c, errors.New("文件设置读取失败"))
			return
		}
		c.JSON(http.StatusOK, value)
	})
	r.engine.PUT("/api/v1/settings/files", func(c *gin.Context) {
		if !localFileRequest(c) {
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		var edit fileopen.Settings
		if err := decoder.Decode(&edit); err != nil {
			fileActionError(c, fileopen.ErrInvalid)
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			fileActionError(c, fileopen.ErrInvalid)
			return
		}
		value, err := s.Save(c.Request.Context(), edit)
		if err != nil {
			fileActionError(c, err)
			return
		}
		c.JSON(http.StatusOK, value)
	})
	r.engine.POST("/api/v1/settings/files/pick-application", func(c *gin.Context) {
		if !localFileRequest(c) {
			return
		}
		app, err := s.PickApplication(c.Request.Context())
		if err != nil {
			fileActionError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"application": app})
	})
	r.engine.POST("/api/v1/files/open", func(c *gin.Context) {
		if !localFileRequest(c) {
			return
		}
		if err := s.Open(c.Request.Context(), c.Query("path")); err != nil {
			fileActionError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	return r
}
