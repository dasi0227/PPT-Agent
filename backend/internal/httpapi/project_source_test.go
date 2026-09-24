package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

func TestProjectSourceWithoutSlides(t *testing.T) {
	srv, _ := setupProjectThreadServer(t)
	created := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"项目文档"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var project model.Project
	if err := json.Unmarshal(created.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	base := srv.URL + "/api/v1/projects/" + project.ID
	for _, kind := range []string{"manifest", "design"} {
		t.Run(kind, func(t *testing.T) {
			url := base + "/source?kind=" + kind
			read := apiReq(t, http.MethodGet, url, "")
			var document service.SlideSourceDocument
			if read.Code != 200 || json.Unmarshal(read.Body.Bytes(), &document) != nil || !document.Writable || document.Path != kind+".json" || document.SlideID != "" {
				t.Fatalf("read: %d %s", read.Code, read.Body.String())
			}
			var content map[string]any
			if err := json.Unmarshal([]byte(document.Content), &content); err != nil {
				t.Fatal(err)
			}
			if kind == "manifest" {
				content["goal"] = "清晰解释下一阶段工作"
			} else {
				content["layout_preferences"] = []string{"优先留白和对齐"}
			}
			raw, _ := json.Marshal(content)
			body := map[string]any{"content": string(raw), "expected_source_hash": document.SourceHash}
			encode := func() string { value, _ := json.Marshal(body); return string(value) }
			missing := apiReq(t, http.MethodPut, url, encode())
			if missing.Code != 400 {
				t.Fatalf("missing scene accepted: %s", missing.Body.String())
			}
			body["expected_scene_revision"] = document.SceneRevision
			saved := apiReq(t, http.MethodPut, url, encode())
			var result struct {
				Changed  bool                        `json:"changed"`
				Document service.SlideSourceDocument `json:"document"`
			}
			if saved.Code != 200 || json.Unmarshal(saved.Body.Bytes(), &result) != nil || !result.Changed || result.Document.SourceHash == document.SourceHash {
				t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
			}
			conflict := apiReq(t, http.MethodPut, url, encode())
			if conflict.Code != 409 {
				t.Fatalf("old hash accepted: %s", conflict.Body.String())
			}
			body["content"], body["expected_source_hash"] = result.Document.Content, result.Document.SourceHash
			before := apiReq(t, http.MethodGet, base+"/history", "")
			noop := apiReq(t, http.MethodPut, url, encode())
			if noop.Code != 200 || json.Unmarshal(noop.Body.Bytes(), &result) != nil || result.Changed {
				t.Fatalf("no-op: %d %s", noop.Code, noop.Body.String())
			}
			after := apiReq(t, http.MethodGet, base+"/history", "")
			if before.Body.String() != after.Body.String() {
				t.Fatal("no-op changed project history")
			}
		})
	}
	invalid := apiReq(t, http.MethodGet, base+"/source?kind=spec", "")
	if invalid.Code != 400 {
		t.Fatalf("page resource without page accepted: %s", invalid.Body.String())
	}
}
