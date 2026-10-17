package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
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
			response = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{"client_request_id":"delete-running","scope":{"selection":{"kind":"all_pages"}},"mode":"execute","instruction":"生成演示文稿"}`)
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
			if response.Code != http.StatusNoContent {
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
			deadline := time.Now().Add(2 * time.Second)
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
