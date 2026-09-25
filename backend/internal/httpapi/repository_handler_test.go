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
	db, cleanup, err := sqlitestore.Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "repository.db")}, zap.NewNop())
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

	engine.GET("/api/v1/themes/:id/css", handler.ThemeCSS)

	engine.GET("/api/v1/components", handler.ListComponents)
	engine.GET("/api/v1/components/:id", handler.GetComponent)

	engine.GET("/api/v1/skills", handler.ListSkills)
	engine.GET("/api/v1/skills/:id", handler.GetSkill)

	resources := NewResourceHandler(service.NewResourceService(workRoot, metadata))
	engine.GET("/api/v1/tags", resources.ListTags)
	engine.POST("/api/v1/resources", resources.Register)
	engine.PATCH("/api/v1/resources/:type/:id", resources.Patch)
	engine.DELETE("/api/v1/resources/:type/:id", resources.Delete)
	engine.POST("/api/v1/snippets", resources.CreateSnippet)
	engine.GET("/api/v1/snippets", resources.ListSnippets)
	engine.GET("/api/v1/snippets/:id", resources.GetSnippet)
	engine.PUT("/api/v1/snippets/:id/content", resources.WriteSnippet)
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

func TestRuntimeResourceCacheValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/resource", func(c *gin.Context) {
		serveRuntimeResource(c, "text/css", []byte("body{}"), c.Query("v") != "")
	})
	initial := performRepositoryRequest(t, engine, http.MethodGet, "/resource?v=hash", "")
	if initial.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || initial.Header().Get("ETag") == "" {
		t.Fatalf("missing versioned caching headers: %v", initial.Header())
	}
	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	request.Header.Set("If-None-Match", initial.Header().Get("ETag"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
		t.Fatalf("unchanged resource resent: %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "public, no-cache" {
		t.Fatalf("unversioned resource must revalidate: %v", response.Header())
	}
}

func TestRepositoryRegistryAndPayloadContracts(t *testing.T) {
	root := t.TempDir()
	engine := repositoryTestRouter(t, root)
	fixtures := []struct{ kind, id, file, body string }{
		{"theme", "theme", "themes/theme/theme.css", completeThemeFixtureCSS()},
		{"component", "card", "components/card/index.html", "<article>Card</article>"},
		{"skill", "story", "skills/story/SKILL.md", "# Pure instructions"},
		{"snippet", "phrase", "snippets/phrase/snippet.txt", "Pure phrase"},
	}
	for _, f := range fixtures {
		writeRepositoryFixture(t, root, "assets/"+f.file, f.body)
		body, _ := json.Marshal(map[string]any{"type": f.kind, "id": f.id, "name": f.id, "description": "Description", "tags": []string{}})
		registered := performRepositoryRequest(t, engine, http.MethodPost, "/api/v1/resources", string(body))
		if registered.Code != 201 {
			t.Fatalf("register: %d %s", registered.Code, registered.Body.String())
		}
		if duplicate := performRepositoryRequest(t, engine, http.MethodPost, "/api/v1/resources", string(body)); duplicate.Code != 409 {
			t.Fatal("duplicate registration accepted")
		}
		patched := performRepositoryRequest(t, engine, http.MethodPatch, "/api/v1/resources/"+f.kind+"/"+f.id, `{"name":"Renamed","disabled":true}`)
		if patched.Code != 200 {
			t.Fatal(patched.Body.String())
		}
		raw, _ := os.ReadFile(filepath.Join(root, "assets", f.file))
		if string(raw) != f.body {
			t.Fatal("metadata changed file")
		}
		if err := os.Remove(filepath.Join(root, "assets", f.file)); err != nil {
			t.Fatal(err)
		}
		plural := f.kind + "s"
		if f.kind == "snippet" {
			plural = "snippets"
		}
		got := performRepositoryRequest(t, engine, http.MethodGet, "/api/v1/"+plural+"/"+f.id, "")
		if got.Code != 200 || !strings.Contains(got.Body.String(), `"content_state":"missing"`) {
			t.Fatalf("missing resource: %d %s", got.Code, got.Body.String())
		}
		deleted := performRepositoryRequest(t, engine, http.MethodDelete, "/api/v1/resources/"+f.kind+"/"+f.id, "")
		if deleted.Code != 204 {
			t.Fatal(deleted.Body.String())
		}
	}
}
func TestThemeCSSVersionTracksOnlyPayload(t *testing.T) {
	root := t.TempDir()
	engine := repositoryTestRouter(t, root)
	path := "assets/themes/test/theme.css"
	css := completeThemeFixtureCSS()
	writeRepositoryFixture(t, root, path, css)
	registered := performRepositoryRequest(t, engine, "POST", "/api/v1/resources", `{"type":"theme","id":"test","name":"Test","description":"Theme"}`)
	if registered.Code != 201 {
		t.Fatal(registered.Body.String())
	}
	first := performRepositoryRequest(t, engine, "GET", "/api/v1/themes/test", "")
	var theme struct {
		StyleHash string `json:"style_hash"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &theme); err != nil {
		t.Fatal(err)
	}
	url := "/api/v1/themes/test/css?v=" + strings.TrimPrefix(theme.StyleHash, "sha256:")
	patched := performRepositoryRequest(t, engine, "PATCH", "/api/v1/resources/theme/test", `{"name":"Renamed","disabled":true}`)
	if patched.Code != 200 {
		t.Fatal(patched.Body.String())
	}
	if got := performRepositoryRequest(t, engine, "GET", url, ""); got.Code != 200 {
		t.Fatal("metadata or disabled state broke existing theme rendering")
	}
	writeRepositoryFixture(t, root, path, css+"\n.card { border-style:dashed; }")
	if got := performRepositoryRequest(t, engine, "GET", url, ""); got.Code != 409 {
		t.Fatal("obsolete style version served")
	}
}

func TestResourceTagsEndpointValidatesScope(t *testing.T) {
	router := repositoryTestRouter(t, t.TempDir())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/tags?scope=snippet", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("tags: %d %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Tags []struct {
			Scope  string `json:"scope"`
			Name   string `json:"name"`
			Key    string `json:"key"`
			System bool   `json:"is_system"`
			Order  int    `json:"sort_order"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Tags) != 6 || response.Tags[0].Key != "identity" || response.Tags[0].Name != "身份" || !response.Tags[0].System {
		t.Fatalf("dictionary: %+v", response.Tags)
	}
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/v1/tags?scope=prompt", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid scope: %d", invalid.Code)
	}
}
