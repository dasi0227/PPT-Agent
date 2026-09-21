package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

// LLMHandler exposes only the safe, user-selectable profile projection.
type LLMHandler struct {
	registry *llm.Registry
}

func NewLLMHandler(registry *llm.Registry) *LLMHandler {
	return &LLMHandler{registry: registry}
}

func (h *LLMHandler) Profiles(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, h.registry.Public())
}

func (h *LLMHandler) Settings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	settings, err := h.registry.Settings()
	if err != nil {
		settingsHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}
func (h *LLMHandler) SaveSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var edit llm.SettingsEdit
	if err := decoder.Decode(&edit); err != nil {
		settingsHTTPError(c, &llm.SettingsError{Code: "SETTINGS_INVALID", Message: "设置格式无效，请检查输入。"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		settingsHTTPError(c, &llm.SettingsError{Code: "SETTINGS_INVALID", Message: "设置必须是单个 JSON 对象。"})
		return
	}
	settings, err := h.registry.SaveSettings(edit)
	if err != nil {
		settingsHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}
func settingsHTTPError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code, message := "SETTINGS_WRITE_FAILED", "模型设置暂时不可用，原设置仍然有效。"
	var settingsErr *llm.SettingsError
	if errors.As(err, &settingsErr) {
		code, message = settingsErr.Code, settingsErr.Message
		switch code {
		case "SETTINGS_INVALID":
			status = http.StatusBadRequest
		case "SETTINGS_REVISION_CONFLICT", "SETTINGS_FILE_CHANGED":
			status = http.StatusConflict
		}
	}
	AbortWithError(c, &APIError{HTTPStatus: status, Code: code, Message: message})
}
