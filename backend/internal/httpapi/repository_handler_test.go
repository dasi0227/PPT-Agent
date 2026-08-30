package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
)

func writeRepositoryFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repositoryTestRouter(root string) *gin.Engine {
	workRoot := service.WorkRoot(root)
	handler := NewRepositoryHandler(
		service.NewThemeService(workRoot),
		service.NewComponentService(workRoot),
		service.NewSkillService(workRoot),
	)
	engine := gin.New()
	engine.GET("/api/v1/runtime/base.css", handler.RuntimeBaseCSS)
	engine.GET("/api/v1/themes", handler.ListThemes)
	engine.GET("/api/v1/themes/:id", handler.GetTheme)
	engine.GET("/api/v1/themes/:id/css", handler.ThemeCSS)
	engine.GET("/api/v1/components", handler.ListComponents)
	engine.GET("/api/v1/components/:id", handler.GetComponent)
	engine.GET("/api/v1/skills", handler.ListSkills)
	engine.GET("/api/v1/skills/:id", handler.GetSkill)
	engine.PATCH("/api/v1/skills/:id", handler.PatchSkill)
	return engine
}

func performRepositoryRequest(t *testing.T, engine http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func TestRepositoryHandlerContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	writeRepositoryFixture(t, root, "assets/themes/swiss-modern/manifest.json", `{"name":"Swiss Modern","description":"Grid","tags":["minimal"]}`)
	writeRepositoryFixture(t, root, "assets/themes/swiss-modern/theme.css", ":root{--color-bg:#fff}")
	writeRepositoryFixture(t, root, "assets/components/feature-card/index.html", `<!doctype html><script id="meta" type="application/json">{"name":"Feature Card","description":"Summary","tags":["card"],"kind":"content"}</script><article>Feature</article>`)
	writeRepositoryFixture(t, root, "skills/story/SKILL.md", "---\nname: Story\ndescription: Narrative\n---\n# Story\n")

	engine := repositoryTestRouter(root)

	base := performRepositoryRequest(t, engine, http.MethodGet, "/api/v1/runtime/base.css", "")
	if base.Code != http.StatusOK || !strings.HasPrefix(base.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("base CSS response = %d %q", base.Code, base.Header().Get("Content-Type"))
	}

	themeCSS := performRepositoryRequest(t, engine, http.MethodGet, "/api/v1/themes/swiss-modern/css", "")
	if themeCSS.Code != http.StatusOK || !strings.HasPrefix(themeCSS.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("theme CSS response = %d %q", themeCSS.Code, themeCSS.Header().Get("Content-Type"))
	}

	component := performRepositoryRequest(t, engine, http.MethodGet, "/api/v1/components/feature-card", "")
	if component.Code != http.StatusOK {
		t.Fatalf("component response = %d %s", component.Code, component.Body.String())
	}
	var componentBody map[string]any
	if err := json.Unmarshal(component.Body.Bytes(), &componentBody); err != nil {
		t.Fatal(err)
	}
	if componentBody["id"] != "feature-card" || componentBody["name"] != "Feature Card" || componentBody["open_url"] == "" {
		t.Fatalf("unexpected component DTO: %#v", componentBody)
	}
	if _, exists := componentBody["disabled"]; exists {
		t.Fatalf("component DTO must be stateless: %#v", componentBody)
	}

	patched := performRepositoryRequest(t, engine, http.MethodPatch, "/api/v1/skills/story", `{"disabled":true}`)
	if patched.Code != http.StatusOK {
		t.Fatalf("skill patch response = %d %s", patched.Code, patched.Body.String())
	}
	var skillBody map[string]any
	if err := json.Unmarshal(patched.Body.Bytes(), &skillBody); err != nil {
		t.Fatal(err)
	}
	if skillBody["id"] != "story" || skillBody["disabled"] != true || skillBody["open_url"] == "" {
		t.Fatalf("unexpected skill DTO: %#v", skillBody)
	}
}
