package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/gin-gonic/gin"
)

func (h *LLMHandler) AgentSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	settings, err := h.registry.AgentSettings()
	if err != nil {
		settingsHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *LLMHandler) ReloadAgentSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if _, err := h.registry.ReloadSettings(); err != nil {
		settingsHTTPError(c, err)
		return
	}
	h.AgentSettings(c)
}

func (h *LLMHandler) SaveAgentSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var edit llm.AgentSettingsEdit
	if err := decoder.Decode(&edit); err != nil {
		settingsHTTPError(c, &llm.SettingsError{Code: "SETTINGS_INVALID", Message: "智能体设置格式无效。"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		settingsHTTPError(c, &llm.SettingsError{Code: "SETTINGS_INVALID", Message: "设置必须是单个 JSON 对象。"})
		return
	}
	settings, err := h.registry.SaveAgentSettings(edit)
	if err != nil {
		settingsHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}
