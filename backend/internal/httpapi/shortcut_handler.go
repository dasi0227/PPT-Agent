package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/dasi0227/PPT-Agent/backend/internal/shortcuts"
	"github.com/gin-gonic/gin"
)

func (r *Router) WithShortcutSettings(s *shortcuts.Service) *Router {
	r.engine.GET("/api/v1/settings/shortcuts", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		value, err := s.Get(c.Request.Context())
		if err != nil {
			AbortWithError(c, &APIError{HTTPStatus: 500, Code: "SHORTCUT_SETTINGS_FAILED", Message: "快捷键设置读取失败"})
			return
		}
		c.JSON(http.StatusOK, value)
	})
	r.engine.PUT("/api/v1/settings/shortcuts", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		var edit shortcuts.Settings
		if err := decoder.Decode(&edit); err != nil {
			AbortWithError(c, &APIError{HTTPStatus: 400, Code: "SHORTCUT_SETTINGS_INVALID", Message: "快捷键配置格式无效"})
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			AbortWithError(c, &APIError{HTTPStatus: 400, Code: "SHORTCUT_SETTINGS_INVALID", Message: "快捷键配置必须是单个对象"})
			return
		}
		if err := shortcuts.Validate(edit.Bindings); err != nil {
			AbortWithError(c, &APIError{HTTPStatus: 400, Code: "SHORTCUT_SETTINGS_INVALID", Message: err.Error()})
			return
		}
		value, err := s.Save(c.Request.Context(), edit)
		if err != nil {
			status, code, message := 500, "SHORTCUT_SETTINGS_FAILED", "快捷键设置保存失败，原配置仍然有效"
			if errors.Is(err, shortcuts.ErrConflict) {
				status, code, message = 409, "SHORTCUT_SETTINGS_CONFLICT", err.Error()
			}
			AbortWithError(c, &APIError{HTTPStatus: status, Code: code, Message: message})
			return
		}
		c.JSON(http.StatusOK, value)
	})
	return r
}
