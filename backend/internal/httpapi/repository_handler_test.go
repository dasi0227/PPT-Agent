package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

func assertRepositoryFileContains(t *testing.T, root, relative string, values ...string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		if !strings.Contains(string(raw), value) {
			t.Fatalf("%s does not contain %q:\n%s", relative, value, raw)
		}
	}
}

func completeThemeFixtureCSS() string {
	var css strings.Builder
	css.WriteString(":root {\n")
	for _, token := range designsystem.RequiredTokens() {
		css.WriteString("  " + token + ": 1;\n")
	}
	css.WriteString("}\n")
	return css.String()
}

func repositoryTestRouter(t *testing.T, root string) *gin.Engine {
	t.Helper()
	workRoot := service.WorkRoot(root)
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "repository.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	metadata, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewRepositoryHandler(
		service.NewThemeService(workRoot, metadata),
		service.NewComponentService(workRoot, metadata),
		service.NewSkillService(workRoot, metadata),
	)
	engine := gin.New()
	engine.GET("/api/v1/runtime/base.css", handler.RuntimeBaseCSS)
	engine.GET("/api/v1/themes", handler.ListThemes)
	engine.GET("/api/v1/themes/:id", handler.GetTheme)
	engine.PATCH("/api/v1/themes/:id", handler.PatchTheme)
	engine.GET("/api/v1/themes/:id/css", handler.ThemeCSS)
	engine.DELETE("/api/v1/themes/:id", handler.DeleteTheme)
	engine.GET("/api/v1/components", handler.ListComponents)
	engine.GET("/api/v1/components/:id", handler.GetComponent)
	engine.PATCH("/api/v1/components/:id", handler.PatchComponent)
	engine.DELETE("/api/v1/components/:id", handler.DeleteComponent)
	engine.GET("/api/v1/skills", handler.ListSkills)
	engine.GET("/api/v1/skills/:id", handler.GetSkill)
	engine.PATCH("/api/v1/skills/:id", handler.PatchSkill)
	engine.DELETE("/api/v1/skills/:id", handler.DeleteSkill)
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
	writeRepositoryFixture(t, root, "assets/themes/swiss-modern/theme.css", "/*\n---\nname: Swiss Modern\ndescription: Grid\n---\n*/\n"+completeThemeFixtureCSS())
	writeRepositoryFixture(t, root, "assets/components/feature-card/index.html", "<!--\n---\nname: Feature Card\ndescription: Summary\n---\n-->\n<article>Feature</article>")
	writeRepositoryFixture(t, root, "assets/skills/story-architect/SKILL.md", "---\nname: Story\ndescription: Narrative\n---\n# Story\n")

	engine := repositoryTestRouter(t, root)

	base := performRepositoryRequest(t, engine, http.MethodGet, "/api/v1/runtime/base.css", "")
	if base.Code != http.StatusOK || !strings.HasPrefix(base.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("base CSS response = %d %q", base.Code, base.Header().Get("Content-Type"))
	}

	themeCSS := performRepositoryRequest(t, engine, http.MethodGet, "/api/v1/themes/swiss-modern/css", "")
	if themeCSS.Code != http.StatusOK || !strings.HasPrefix(themeCSS.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("theme CSS response = %d %q", themeCSS.Code, themeCSS.Header().Get("Content-Type"))
	}
	theme := performRepositoryRequest(t, engine, http.MethodGet, "/api/v1/themes/swiss-modern", "")
	var themeBody map[string]any
	if theme.Code != http.StatusOK || json.Unmarshal(theme.Body.Bytes(), &themeBody) != nil {
		t.Fatalf("theme response = %d %s", theme.Code, theme.Body.String())
	}
	if tags, exists := themeBody["tags"].([]any); !exists || len(tags) != 1 || tags[0] != "minimal" {
		t.Fatalf("theme DTO tags = %#v", themeBody["tags"])
	}
	themePatched := performRepositoryRequest(t, engine, http.MethodPatch, "/api/v1/themes/swiss-modern", `{"name":"Swiss Edited","description":"Edited theme","tags":["business"]}`)
	if themePatched.Code != http.StatusOK || !strings.Contains(themePatched.Body.String(), `"business"`) || !strings.Contains(themePatched.Body.String(), `"Swiss Edited"`) {
		t.Fatalf("theme patch response = %d %s", themePatched.Code, themePatched.Body.String())
	}
	assertRepositoryFileContains(t, root, "assets/themes/swiss-modern/theme.css", "name: Swiss Edited", "description: Edited theme", "--color-bg")

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
	if componentBody["disabled"] != false {
		t.Fatalf("component must default to enabled: %#v", componentBody)
	}
	if _, exists := componentBody["kind"]; exists {
		t.Fatalf("component DTO must not expose legacy kind: %#v", componentBody)
	}
	componentPatched := performRepositoryRequest(t, engine, http.MethodPatch, "/api/v1/components/feature-card", `{"disabled":true}`)
	if componentPatched.Code != http.StatusOK || !strings.Contains(componentPatched.Body.String(), `"disabled":true`) {
		t.Fatalf("component patch response = %d %s", componentPatched.Code, componentPatched.Body.String())
	}
	componentTagsPatched := performRepositoryRequest(t, engine, http.MethodPatch, "/api/v1/components/feature-card", `{"name":"Feature Edited","description":"Edited component","tags":["list"]}`)
	if componentTagsPatched.Code != http.StatusOK || !strings.Contains(componentTagsPatched.Body.String(), `"list"`) || !strings.Contains(componentTagsPatched.Body.String(), `"Feature Edited"`) {
		t.Fatalf("component tags patch response = %d %s", componentTagsPatched.Code, componentTagsPatched.Body.String())
	}
	assertRepositoryFileContains(t, root, "assets/components/feature-card/index.html", "name: Feature Edited", "description: Edited component", "<article>Feature</article>")

	patched := performRepositoryRequest(t, engine, http.MethodPatch, "/api/v1/skills/story-architect", `{"disabled":true}`)
	if patched.Code != http.StatusOK {
		t.Fatalf("skill patch response = %d %s", patched.Code, patched.Body.String())
	}
	var skillBody map[string]any
	if err := json.Unmarshal(patched.Body.Bytes(), &skillBody); err != nil {
		t.Fatal(err)
	}
	if skillBody["id"] != "story-architect" || skillBody["disabled"] != true || skillBody["open_url"] == "" {
		t.Fatalf("unexpected skill DTO: %#v", skillBody)
	}
	skillTagsPatched := performRepositoryRequest(t, engine, http.MethodPatch, "/api/v1/skills/story-architect", `{"name":"Story Edited","description":"Edited skill","tags":["workflow"]}`)
	if skillTagsPatched.Code != http.StatusOK || !strings.Contains(skillTagsPatched.Body.String(), `"workflow"`) || !strings.Contains(skillTagsPatched.Body.String(), `"Story Edited"`) {
		t.Fatalf("skill tags patch response = %d %s", skillTagsPatched.Code, skillTagsPatched.Body.String())
	}
	assertRepositoryFileContains(t, root, "assets/skills/story-architect/SKILL.md", "name: Story Edited", "description: Edited skill", "# Story")

	for _, deletion := range []struct {
		path string
		dir  string
	}{
		{path: "/api/v1/themes/swiss-modern", dir: "assets/themes/swiss-modern"},
		{path: "/api/v1/components/feature-card", dir: "assets/components/feature-card"},
		{path: "/api/v1/skills/story-architect", dir: "assets/skills/story-architect"},
	} {
		response := performRepositoryRequest(t, engine, http.MethodDelete, deletion.path, "")
		if response.Code != http.StatusNoContent {
			t.Fatalf("delete %s response = %d %s", deletion.path, response.Code, response.Body.String())
		}
		if _, err := os.Stat(filepath.Join(root, deletion.dir)); !os.IsNotExist(err) {
			t.Fatalf("repository directory still exists after deleting %s: %v", deletion.path, err)
		}
	}
}

func TestThemeHandlerRejectsIncompleteTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	writeRepositoryFixture(t, root, "assets/themes/incomplete/theme.css", "/*\n---\nname: Incomplete\ndescription: Missing tokens\n---\n*/\n:root { --color-bg: #fff; }")

	engine := repositoryTestRouter(t, root)
	response := performRepositoryRequest(t, engine, http.MethodGet, "/api/v1/themes/incomplete/css", "")
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "--color-fg") {
		t.Fatalf("incomplete theme response = %d %s", response.Code, response.Body.String())
	}
}
