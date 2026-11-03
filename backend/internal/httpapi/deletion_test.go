package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type deletionBlockedExecution struct{ started, canceled, exit chan struct{} }

func (e deletionBlockedExecution) Run(ctx context.Context, _ workflow.EventEmitter, _ run.Checkpointer, _ run.Prompter) workflow.StructuredOutcome {
	close(e.started)
	<-ctx.Done()
	close(e.canceled)
	<-e.exit
	return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
}

func TestDeletionAcceptsWhileCanceledRunStillExits(t *testing.T) {
	for _, kind := range []string{"thread", "project"} {
		t.Run(kind, func(t *testing.T) {
			execution := deletionBlockedExecution{make(chan struct{}), make(chan struct{}), make(chan struct{})}
			srv, root := setupProjectThreadServerWithFactory(t, func(model.Run, model.CreateRunParams, model.Project) run.Execution { return execution })
			var release sync.Once
			defer release.Do(func() { close(execution.exit) })
			var project, thread map[string]any
			response := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Delete active work"}`)
			if response.Code != http.StatusCreated {
				t.Fatal(response.Body.String())
			}
			if err := json.Unmarshal(response.Body.Bytes(), &project); err != nil {
				t.Fatal(err)
			}
			projectID := project["id"].(string)
			response = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{}`)
			if response.Code != http.StatusCreated {
				t.Fatal(response.Body.String())
			}
			if err := json.Unmarshal(response.Body.Bytes(), &thread); err != nil {
				t.Fatal(err)
			}
			threadID := thread["id"].(string)
			response = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{"scene_revision":"1","client_request_id":"delete-running","scope":{"selection":{"kind":"all_pages"}},"mode":"create","instruction":"生成演示文稿"}`)
			if response.Code != http.StatusCreated {
				t.Fatal(response.Body.String())
			}
			select {
			case <-execution.started:
			case <-time.After(2 * time.Second):
				t.Fatal("run did not start")
			}
			path := "/threads/" + threadID
			if kind == "project" {
				path = "/projects/" + projectID
			}
			response = apiReq(t, http.MethodDelete, srv.URL+"/api/v1"+path, "")
			if response.Code != http.StatusOK {
				t.Fatalf("active run blocked deletion: %d %s", response.Code, response.Body.String())
			}
			if response = apiReq(t, http.MethodGet, srv.URL+"/api/v1"+path, ""); response.Code != http.StatusNotFound {
				t.Fatal("deleted target remains accessible")
			}
			select {
			case <-execution.canceled:
			case <-time.After(2 * time.Second):
				t.Fatal("deletion did not cancel execution")
			}
			if kind == "thread" {
				if response = apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID, ""); response.Code != http.StatusOK {
					t.Fatal("thread deletion removed its project")
				}
			}
			release.Do(func() { close(execution.exit) })
			deletedPath := filepath.Join(root, "projects", projectID)
			if kind == "thread" {
				deletedPath = filepath.Join(deletedPath, "threads", threadID)
			}
			deadline := time.Now().Add(7 * time.Second)
			for {
				if _, err := os.Stat(deletedPath); os.IsNotExist(err) {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("deleted target was not cleaned after execution exited")
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}

func TestProjectAndThreadDeletionOfferUndoAndRejectStaleTokens(t *testing.T) {
	for _, kind := range []string{"project", "thread"} {
		t.Run(kind, func(t *testing.T) {
			srv, root := setupProjectThreadServer(t)
			var project, thread struct {
				ID string `json:"id"`
			}
			response := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Undo deletion"}`)
			if response.Code != http.StatusCreated {
				t.Fatal(response.Body.String())
			}
			if err := json.Unmarshal(response.Body.Bytes(), &project); err != nil {
				t.Fatal(err)
			}
			response = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+project.ID+"/threads", `{}`)
			if response.Code != http.StatusCreated {
				t.Fatal(response.Body.String())
			}
			if err := json.Unmarshal(response.Body.Bytes(), &thread); err != nil {
				t.Fatal(err)
			}
			artifact := filepath.Join(root, "projects", project.ID, "artifacts", "retained.txt")
			if err := os.WriteFile(artifact, []byte("retain all project content"), 0600); err != nil {
				t.Fatal(err)
			}
			id, path := project.ID, "/projects/"+project.ID
			if kind == "thread" {
				id, path = thread.ID, "/threads/"+thread.ID
			}
			deleteTarget := func() string {
				t.Helper()
				response := apiReq(t, http.MethodDelete, srv.URL+"/api/v1"+path, "")
				var receipt struct {
					Deletion struct {
						Token     string `json:"token"`
						UndoUntil int64  `json:"undo_until"`
					} `json:"deletion"`
					ServerNow int64 `json:"server_now"`
				}
				if response.Code != http.StatusOK {
					t.Fatalf("delete: %d %s", response.Code, response.Body.String())
				}
				if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
					t.Fatal(err)
				}
				if receipt.Deletion.Token == "" || receipt.Deletion.UndoUntil <= receipt.ServerNow || receipt.Deletion.UndoUntil > receipt.ServerNow+5000 {
					t.Fatalf("invalid undo window: %+v", receipt)
				}
				if response := apiReq(t, http.MethodGet, srv.URL+"/api/v1"+path, ""); response.Code != http.StatusNotFound {
					t.Fatal("pending target still visible")
				}
				pending := apiReq(t, http.MethodGet, srv.URL+"/api/v1/deletions", "")
				if pending.Code != http.StatusOK || !bytes.Contains(pending.Body.Bytes(), []byte(receipt.Deletion.Token)) {
					t.Fatalf("refresh lost undo entry: %s", pending.Body.String())
				}
				if _, err := os.Stat(artifact); err != nil {
					t.Fatalf("content removed before expiry: %v", err)
				}
				return receipt.Deletion.Token
			}
			undo := func(token string) *httptest.ResponseRecorder {
				return apiReq(t, http.MethodPost, srv.URL+"/api/v1/deletions/"+kind+"/"+id+"/undo", `{"token":"`+token+`"}`)
			}
			first := deleteTarget()
			if response := undo(first); response.Code != http.StatusNoContent {
				t.Fatalf("undo: %d %s", response.Code, response.Body.String())
			}
			if response := undo(first); response.Code != http.StatusNoContent {
				t.Fatalf("replayed undo: %d %s", response.Code, response.Body.String())
			}
			second := deleteTarget()
			if response := undo(first); response.Code != http.StatusConflict {
				t.Fatalf("stale undo: %d %s", response.Code, response.Body.String())
			}
			if response := undo(second); response.Code != http.StatusNoContent {
				t.Fatalf("new undo: %d %s", response.Code, response.Body.String())
			}
			if response := apiReq(t, http.MethodGet, srv.URL+"/api/v1"+path, ""); response.Code != http.StatusOK {
				t.Fatal("restored target unavailable")
			}
			if response := apiReq(t, http.MethodGet, srv.URL+"/api/v1/threads/"+thread.ID+"/view", ""); response.Code != http.StatusOK {
				t.Fatal("restored conversation unavailable")
			}
			if raw, err := os.ReadFile(artifact); err != nil || string(raw) != "retain all project content" {
				t.Fatalf("undo lost content: %q %v", raw, err)
			}
		})
	}
}
