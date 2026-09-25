package httpapi

import (
	"encoding/json"
	"testing"
)

func TestSnippetCRUDUsesFileContentAndResourceMetadata(t *testing.T) {
	engine := repositoryTestRouter(t, t.TempDir())
	created := performRepositoryRequest(t, engine, "POST", "/api/v1/snippets", `{"name":"Phrase","description":"Description","content":"Exact body\n","tags":["other"]}`)
	if created.Code != 201 {
		t.Fatal(created.Body.String())
	}
	var v struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		OpenURL string `json:"open_url"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &v); err != nil || v.Content != "Exact body\n" || v.OpenURL == "" {
		t.Fatalf("created: %s", created.Body.String())
	}
	if got := performRepositoryRequest(t, engine, "POST", "/api/v1/snippets", `{"name":"PHRASE","description":"Description","content":"duplicate"}`); got.Code != 409 {
		t.Fatal("duplicate name accepted")
	}
	if got := performRepositoryRequest(t, engine, "PUT", "/api/v1/snippets/"+v.ID+"/content", `{"content":"Replacement"}`); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	if got := performRepositoryRequest(t, engine, "PATCH", "/api/v1/resources/snippet/"+v.ID, `{"description":"Updated","tags":["workflow"]}`); got.Code != 400 {
		t.Fatal("cross-type tag accepted")
	}
	if got := performRepositoryRequest(t, engine, "DELETE", "/api/v1/resources/snippet/"+v.ID, ""); got.Code != 204 {
		t.Fatal(got.Body.String())
	}
	if got := performRepositoryRequest(t, engine, "GET", "/api/v1/snippets/"+v.ID, ""); got.Code != 404 {
		t.Fatal("deleted snippet exists")
	}
}
