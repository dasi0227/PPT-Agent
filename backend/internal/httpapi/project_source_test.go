package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
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
	for _, kind := range []string{"manifest", "design", "spec"} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			result := apiReq(t, method, base+"/source?kind="+kind, "")
			if result.Code != http.StatusNotFound {
				t.Fatalf("removed endpoint: %d %s", result.Code, result.Body.String())
			}
		}
	}
	removedSave := apiReq(t, http.MethodPut, base+"/slides/sli_any/source?kind=html", `{}`)
	if removedSave.Code != http.StatusNotFound {
		t.Fatalf("HTML source save still routed: %d", removedSave.Code)
	}
	manualHTML := apiReq(t, http.MethodPost, base+"/mutations", `{"op":"slide.html.write","slide_id":"sli_any","html":"<html>edited</html>"}`)
	if manualHTML.Code != http.StatusBadRequest {
		t.Fatalf("manual HTML mutation accepted: %d %s", manualHTML.Code, manualHTML.Body.String())
	}

	current := apiReq(t, http.MethodGet, base+"/content", "")
	var snapshot spec.ProjectContentSnapshot
	if current.Code != http.StatusOK {
		t.Fatal(current.Body.String())
	}
	if err := json.Unmarshal(current.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(map[string]any{"op": "manifest.patch", "expected_hash": snapshot.Hashes["manifest"], "patch": []map[string]any{{"op": "replace", "path": "/pages", "value": "11-12"}}})
	for _, revision := range []int64{snapshot.SceneRevision + 1, snapshot.SceneRevision} {
		req, err := http.NewRequest(http.MethodPost, base+"/mutations", strings.NewReader(string(request)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Expected-Scene-Revision", strconv.FormatInt(revision, 10))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		want := http.StatusOK
		if revision != snapshot.SceneRevision {
			want = http.StatusConflict
		}
		if res.StatusCode != want {
			t.Fatalf("scene %d: %d %s", revision, res.StatusCode, raw)
		}
		if want == http.StatusOK {
			var saved struct {
				Content spec.ProjectContentSnapshot `json:"content"`
			}
			if err := json.Unmarshal(raw, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Content.Manifest.Pages != "11-12" || len(spec.FlattenOutline(saved.Content.Outline)) != 0 {
				t.Fatalf("page requirement not saved independently of outline: %+v", saved)
			}
		}
	}
}
