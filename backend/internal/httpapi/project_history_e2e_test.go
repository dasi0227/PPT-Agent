package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func historyRequest(t *testing.T, srv *httptest.Server, method, path, body string, revision int64) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+"/api/v1"+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if revision != 0 {
		req.Header.Set("X-Discard-Future-Revision", fmt.Sprint(revision))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, raw
}
func TestProjectHistoryHTTPConfirmationAndBranch(t *testing.T) {
	srv, _ := setupProjectThreadServer(t)
	code, raw := historyRequest(t, srv, "POST", "/projects", `{"topic":"Checkpoint"}`, 0)
	if code != 201 {
		t.Fatalf("create: %d %s", code, raw)
	}
	var p struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &p)
	code, raw = historyRequest(t, srv, "POST", "/projects/"+p.ID+"/threads", `{"title":"chat"}`, 0)
	if code != 201 {
		t.Fatal(string(raw))
	}
	var thread struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &thread)
	payload := `{"client_request_id":"cp1","instruction":"hello","mode":"chat","scope":{"selection":{"kind":"all_pages"}}}`
	code, raw = historyRequest(t, srv, "POST", "/threads/"+thread.ID+"/runs", payload, 0)
	if code != 201 {
		t.Fatalf("run: %d %s", code, raw)
	}
	var run struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &run)
	var state struct {
		Revision    int64  `json:"revision"`
		Latest      string `json:"latest"`
		Checkpoints []any  `json:"checkpoints"`
	}
	for i := 0; i < 100; i++ {
		_, raw = historyRequest(t, srv, "GET", "/projects/"+p.ID+"/history", "", 0)
		_ = json.Unmarshal(raw, &state)
		if len(state.Checkpoints) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(state.Checkpoints) != 1 {
		t.Fatal("terminal run checkpoint missing")
	}
	_, raw = historyRequest(t, srv, "GET", "/projects/"+p.ID+"/history/preview?run_id="+run.ID, "", 0)
	var preview struct {
		Revision int64 `json:"revision"`
	}
	_ = json.Unmarshal(raw, &preview)
	switchBody := fmt.Sprintf(`{"run_id":%q,"revision":%d,"operation_id":"rollback1","scene":{"composer":{"threadDrafts":{"%s":"old draft"}}}}`, run.ID, preview.Revision, thread.ID)
	code, raw = historyRequest(t, srv, "POST", "/projects/"+p.ID+"/history/switch", switchBody, 0)
	if code != 200 {
		t.Fatalf("switch: %d %s", code, raw)
	}
	_ = json.Unmarshal(raw, &state)
	code, raw = historyRequest(t, srv, "GET", "/runs/"+run.ID+"/events", "", 0)
	if code == 200 {
		t.Fatal("hidden SSE history visible")
	}
	code, raw = historyRequest(t, srv, "POST", "/projects/"+p.ID+"/threads", `{"title":"branch"}`, 0)
	if code != 409 || !strings.Contains(string(raw), "HISTORY_CONFIRM_REQUIRED") {
		t.Fatalf("confirmation: %d %s", code, raw)
	}
	code, raw = historyRequest(t, srv, "POST", "/threads/"+thread.ID+"/runs", `{"instruction":"invalid"}`, state.Revision)
	if code < 400 {
		t.Fatal("invalid accepted")
	}
	_, raw = historyRequest(t, srv, "GET", "/projects/"+p.ID+"/history", "", 0)
	_ = json.Unmarshal(raw, &state)
	if state.Latest == "" {
		t.Fatal("failed validation lost future")
	}
	code, raw = historyRequest(t, srv, "POST", "/projects/"+p.ID+"/threads", `{"title":"branch"}`, state.Revision)
	if code != 201 {
		t.Fatalf("branch: %d %s", code, raw)
	}
	code, raw = historyRequest(t, srv, "GET", "/projects/"+p.ID+"/history/preview", "", 0)
	if code != 409 {
		t.Fatalf("old future restored: %d %s", code, raw)
	}
}
