package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func setupAssetServer(t *testing.T) (*httptest.Server, *sqlitestore.Store) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(root, "assets.db"), WorkRoot: root}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	runSvc := service.NewRunServiceWithFactory(st, engine, func(model.Run, model.CreateRunParams, model.Project) run.Runner {
		return noOpRunner{}
	})
	router := httpapi.NewRouter(
		cfg,
		zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewAssetHandler(service.NewAssetService(st, root)),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, st
}

type noOpRunner struct{}

func (noOpRunner) Run(context.Context, harness.Emitter, harness.Checkpointer, run.Prompter) harness.Outcome {
	return harness.Outcome{Status: harness.OutcomeFinished}
}

func TestAssetsCRUDREST(t *testing.T) {
	srv, st := setupAssetServer(t)

	createBody := `{
  "manifest":{
    "name":"api-card","version":"1.0.0","kind":"component","source":"preset",
    "description":"API card","tags":["card"],"mount":{"target":".slide-content","position":"append"},
    "assets":{"html":"template.html","css":"style.css"}
  },
  "payload":{
    "template.html":"<div class=\"api-card\">标题</div>",
    "style.css":".api-card{border-radius: 8px;color:var(--color-primary);}"
  }
}`
	resp := assetReq(t, http.MethodPost, srv.URL+"/api/v1/assets", createBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /assets want 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var created map[string]any
	json.Unmarshal(resp.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("empty asset id in response: %s", resp.Body.String())
	}
	if created["source"] != "user" {
		t.Fatalf("POST must force source=user, got %v", created["source"])
	}

	resp = assetReq(t, http.MethodGet, srv.URL+"/api/v1/assets?kind=component", "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "api-card") {
		t.Fatalf("GET /assets?kind=component failed: %d %s", resp.Code, resp.Body.String())
	}

	resp = assetReq(t, http.MethodGet, srv.URL+"/api/v1/assets/"+id, "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"name":"api-card"`) {
		t.Fatalf("GET /assets/{id} failed: %d %s", resp.Code, resp.Body.String())
	}

	patchBody := `{"edits":[{"file":"style.css","old_text":"border-radius: 8px","new_text":"border-radius: 24px"}]}`
	resp = assetReq(t, http.MethodPatch, srv.URL+"/api/v1/assets/"+id, patchBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH /assets/{id} want 200, got %d: %s", resp.Code, resp.Body.String())
	}
	versions, err := st.ListVersions(context.Background(), "asset", "asset-"+id)
	if err != nil {
		t.Fatalf("list asset versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("POST+PATCH should produce 2 asset versions, got %+v", versions)
	}

	resp = assetReq(t, http.MethodDelete, srv.URL+"/api/v1/assets/"+id, "")
	if resp.Code != http.StatusNoContent {
		t.Fatalf("DELETE /assets/{id} want 204, got %d: %s", resp.Code, resp.Body.String())
	}
	resp = assetReq(t, http.MethodGet, srv.URL+"/api/v1/assets/"+id, "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("GET deleted asset want 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDuplicateAssetCreateReturnsConflictREST(t *testing.T) {
	srv, _ := setupAssetServer(t)
	body := `{
  "manifest":{
    "name":"dupe-api-card","version":"1.0.0","kind":"component",
    "description":"API card","mount":{"target":".slide-content","position":"append"},
    "assets":{"html":"template.html","css":"style.css"}
  },
  "payload":{
    "template.html":"<div class=\"dupe-api-card\">标题</div>",
    "style.css":".dupe-api-card{color:var(--color-primary);}"
  }
}`
	first := assetReq(t, http.MethodPost, srv.URL+"/api/v1/assets", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first create want 201, got %d: %s", first.Code, first.Body.String())
	}
	second := assetReq(t, http.MethodPost, srv.URL+"/api/v1/assets", body)
	if second.Code != http.StatusConflict {
		t.Fatalf("duplicate create want 409, got %d: %s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), `"code":"CONFLICT"`) {
		t.Fatalf("expected CONFLICT body, got %s", second.Body.String())
	}
}

func TestAssetRollbackREST(t *testing.T) {
	srv, st := setupAssetServer(t)
	body := `{
  "manifest":{
    "name":"rollback-card","version":"1.0.0","kind":"component",
    "description":"rollback card","mount":{"target":".slide-content","position":"append"},
    "assets":{"html":"template.html","css":"style.css"}
  },
  "payload":{
    "template.html":"<div class=\"rollback-card\">标题</div>",
    "style.css":".rollback-card{border-radius: 8px;color:var(--color-primary);}"
  }
}`
	resp := assetReq(t, http.MethodPost, srv.URL+"/api/v1/assets", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var created map[string]any
	json.Unmarshal(resp.Body.Bytes(), &created)
	id := created["id"].(string)
	patch := `{"edits":[{"file":"style.css","old_text":"border-radius: 8px","new_text":"border-radius: 24px"}]}`
	resp = assetReq(t, http.MethodPatch, srv.URL+"/api/v1/assets/"+id, patch)
	if resp.Code != http.StatusOK {
		t.Fatalf("patch want 200, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = assetReq(t, http.MethodPost, srv.URL+"/api/v1/assets/"+id+"/rollback", `{"version_no":0}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("rollback want 200, got %d: %s", resp.Code, resp.Body.String())
	}
	versions, err := st.ListVersions(context.Background(), "asset", "asset-"+id)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 3 || versions[2].VersionNo != 2 {
		t.Fatalf("rollback should append v2, got %+v", versions)
	}
	missing := assetReq(t, http.MethodPost, srv.URL+"/api/v1/assets/"+id+"/rollback", `{"version_no":99}`)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("rollback missing version want 404, got %d: %s", missing.Code, missing.Body.String())
	}
}

func TestCreateAssetValidationFailedREST(t *testing.T) {
	srv, _ := setupAssetServer(t)
	body := `{"manifest":{"name":"bad","version":"1.0.0","kind":"component","description":"bad","assets":{}},"payload":{}}`
	resp := assetReq(t, http.MethodPost, srv.URL+"/api/v1/assets", body)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"code":"VALIDATION_FAILED"`) {
		t.Fatalf("expected VALIDATION_FAILED body, got %s", resp.Body.String())
	}
}

func TestListAssetsInvalidKindREST(t *testing.T) {
	srv, _ := setupAssetServer(t)
	resp := assetReq(t, http.MethodGet, srv.URL+"/api/v1/assets?kind=bad", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid kind, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"code":"BAD_REQUEST"`) {
		t.Fatalf("expected BAD_REQUEST body, got %s", resp.Body.String())
	}
}

func assetReq(t *testing.T, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	rr.Code = resp.StatusCode
	_, _ = rr.Body.ReadFrom(resp.Body)
	return rr
}
