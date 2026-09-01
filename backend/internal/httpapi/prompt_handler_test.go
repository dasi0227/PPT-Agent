package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func promptTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(t.TempDir(), "handler.db")}, zap.NewNop())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	handler := NewPromptHandler(service.NewPromptService(st))
	engine := gin.New()
	engine.GET("/api/v1/prompts", handler.List)
	engine.GET("/api/v1/prompts/:id", handler.Get)
	engine.POST("/api/v1/prompts", handler.Create)
	engine.PUT("/api/v1/prompts/:id", handler.Update)
	engine.DELETE("/api/v1/prompts/:id", handler.Delete)
	return engine
}

func promptRequest(t *testing.T, engine http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func TestPromptHandlerCRUDAndErrors(t *testing.T) {
	engine := promptTestEngine(t)
	body := map[string]any{
		"key_zh": "高管摘要", "key_en": "executive-summary",
		"value": "生成摘要", "tags": []string{"summarize"},
	}
	created := promptRequest(t, engine, http.MethodPost, "/api/v1/prompts", body)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var prompt struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &prompt); err != nil || prompt.ID == "" {
		t.Fatalf("decode prompt: %v, %+v", err, prompt)
	}

	conflictBody := map[string]any{
		"key_zh": "其他摘要", "key_en": "EXECUTIVE-SUMMARY",
		"value": "冲突", "tags": []string{},
	}
	conflict := promptRequest(t, engine, http.MethodPost, "/api/v1/prompts", conflictBody)
	if conflict.Code != http.StatusConflict || !bytes.Contains(conflict.Body.Bytes(), []byte(`"field":"key_en"`)) {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}

	invalid := promptRequest(t, engine, http.MethodPost, "/api/v1/prompts", map[string]any{
		"key_zh": "english", "key_en": "valid", "value": "bad", "tags": []string{},
	})
	if invalid.Code != http.StatusBadRequest || !bytes.Contains(invalid.Body.Bytes(), []byte(`"code":"PROMPT_INVALID"`)) {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}

	deleted := promptRequest(t, engine, http.MethodDelete, "/api/v1/prompts/"+prompt.ID, nil)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	missing := promptRequest(t, engine, http.MethodGet, "/api/v1/prompts/"+prompt.ID, nil)
	if missing.Code != http.StatusNotFound || !bytes.Contains(missing.Body.Bytes(), []byte(`"code":"PROMPT_NOT_FOUND"`)) {
		t.Fatalf("missing status=%d body=%s", missing.Code, missing.Body.String())
	}
}
