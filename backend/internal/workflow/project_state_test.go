package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func sourcesFromFiles(t *testing.T, files map[string][]byte) *projectSources {
	t.Helper()
	s, err := readProjectSources(func(name string) ([]byte, error) {
		if b, ok := files[name]; ok {
			return b, nil
		}
		return nil, os.ErrNotExist
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func outlineSource(t *testing.T, slides ...spec.SlideNode) []byte {
	t.Helper()
	out := spec.Outline{Sections: []spec.Section{{ID: "sec_test", Title: "Test", Purpose: "Test", Slides: slides, Subsections: []spec.Subsection{}}}}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestProjectStateAvailabilityAndReadFailures(t *testing.T) {
	empty := sourcesFromFiles(t, nil).state(nil, nil)
	if empty.Outline != "missing" || empty.Manifest != "missing" || empty.Design != "missing" || empty.Slides == nil || empty.Changes == nil {
		t.Fatal(empty)
	}
	files := map[string][]byte{
		".outline.json": outlineSource(t, spec.SlideNode{ID: "sli_a", Title: "A"}, spec.SlideNode{ID: "sli_b", Title: "B"}, spec.SlideNode{ID: "sli_c", Title: "C"}),
		".spec.json":    []byte(`{"sli_a":{"core":"A","elements":[]}}`),
		"sli_a.html":    []byte("<p>A</p>"),
	}
	s := sourcesFromFiles(t, files)
	state := s.state([]RenderedImageContext{{SlideID: "sli_a"}, {SlideID: "sli_b", Stale: true}}, s)
	if state.Slides["sli_a"] != (SlideState{"exists", "exists", "fresh"}) || state.Slides["sli_b"] != (SlideState{"missing", "missing", "stale"}) || state.Slides["sli_c"].Render != "missing" {
		t.Fatal(state)
	}
	for _, name := range []string{".outline.json", ".manifest.json", ".design.json", ".spec.json", "sli_a.html"} {
		_, err := readProjectSources(func(path string) ([]byte, error) {
			if path == name {
				return nil, os.ErrPermission
			}
			if b, ok := files[path]; ok {
				return b, nil
			}
			return nil, os.ErrNotExist
		})
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("%s read failure concealed: %v", name, err)
		}
	}
	for _, name := range []string{".outline.json", ".manifest.json", ".design.json", ".spec.json"} {
		_, err := readProjectSources(func(path string) ([]byte, error) {
			if path == name {
				return []byte(`{"broken":`), nil
			}
			if b, ok := files[path]; ok {
				return b, nil
			}
			return nil, os.ErrNotExist
		})
		if err == nil {
			t.Fatalf("%s corruption treated as missing", name)
		}
	}
}

func TestProjectNetChangesAggregateIdentityAndReversion(t *testing.T) {
	baselineFiles := map[string][]byte{".outline.json": outlineSource(t, spec.SlideNode{ID: "sli_a", Title: "A"})}
	baseline := sourcesFromFiles(t, baselineFiles)
	assert := func(files map[string][]byte, want []ProjectChange) {
		t.Helper()
		got := projectNetChanges(baseline, sourcesFromFiles(t, files))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	// First content for an existing slide is an update, regardless of resource part.
	files := map[string][]byte{".outline.json": baselineFiles[".outline.json"], ".spec.json": []byte(`{"sli_a":{"core":"first","elements":[]}}`), "sli_a.html": []byte("first")}
	assert(files, []ProjectChange{{"sli_a", "updated"}})
	files[".outline.json"] = outlineSource(t, spec.SlideNode{ID: "sli_a", Title: "Renamed"}, spec.SlideNode{ID: "sli_b", Title: "New"})
	files["sli_b.html"] = []byte("new html")
	assert(files, []ProjectChange{{"outline", "updated"}, {"sli_a", "updated"}, {"sli_b", "created"}})
	files["sli_b.html"] = []byte("updated new html")
	assert(files, []ProjectChange{{"outline", "updated"}, {"sli_a", "updated"}, {"sli_b", "created"}})
	// Creation followed by deletion vanishes, even when orphan output remains.
	files[".outline.json"] = baselineFiles[".outline.json"]
	delete(files, ".spec.json")
	delete(files, "sli_a.html")
	assert(files, []ProjectChange{})
	// Empty HTML is still a real newly existing file.
	files["sli_a.html"] = []byte{}
	assert(files, []ProjectChange{{"sli_a", "updated"}})
	delete(files, "sli_a.html")
	files[".outline.json"] = outlineSource(t, spec.SlideNode{ID: "sli_b", Title: "New"})
	assert(files, []ProjectChange{{"outline", "updated"}, {"sli_a", "deleted"}, {"sli_b", "created"}})
}

func TestProjectSnapshotResumeUsesOriginalBaselineAndCurrentSources(t *testing.T) {
	root, _, pack := generationPackFixture(t)
	runID := "snapshot-resume"
	if err := ensureReviewBaseline(context.Background(), root, runID, false); err != nil {
		t.Fatal(err)
	}
	state := &RunState{projectDir: root, runID: runID, pack: pack}
	if err := state.refreshProjectState(); err != nil {
		t.Fatal(err)
	}
	if len(state.projectState.Changes) != 0 {
		t.Fatal("initial changes")
	}
	original, err := os.ReadFile(filepath.Join(root, model.SlideHTMLPath(generationSlide)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, model.SlideHTMLPath(generationSlide)), []byte("<h1>Changed</h1>"), 0600); err != nil {
		t.Fatal(err)
	}
	restored := &RunState{projectDir: root, runID: runID, pack: pack}
	if err := restored.refreshProjectState(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.projectState.Changes, []ProjectChange{{generationSlide, "updated"}}) {
		t.Fatal(restored.projectState)
	}
	if got := restored.pack.SlideHTML.Summaries[generationSlide].TextDigest; len(got) == 0 || got[0] != "Changed" {
		t.Fatal("stale retrieval source")
	}
	if err := os.WriteFile(filepath.Join(root, model.SlideHTMLPath(generationSlide)), original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := restored.refreshProjectState(); err != nil {
		t.Fatal(err)
	}
	if len(restored.projectState.Changes) != 0 {
		t.Fatal("revert did not clear changes")
	}
	// A missing durable baseline is an error, not a newly chosen baseline.
	restored.projectBaseline = nil
	if err := os.Remove(reviewBaselinePath(root, runID)); err != nil {
		t.Fatal(err)
	}
	if err := restored.refreshProjectState(); err == nil {
		t.Fatal("silently reset run baseline")
	}
}

func TestPlanProjectionRequiresMatchingApprovalAndKeepsStepIDs(t *testing.T) {
	p := &Plan{ID: "private", ApprovalID: "approved", Title: "Plan", Content: "Body", Status: PlanActive, Steps: []PlanStep{{ID: "step_actual", Title: "Step", Status: PlanStepPending}}}
	approved := func() bool {
		t.Helper()
		messages := runtimeContextMessages(AgentRequest{Plan: p})
		var data map[string]any
		if err := json.Unmarshal([]byte(runtimeBody(contextSectionText(messages, "plan"))), &data); err != nil {
			t.Fatal(err)
		}
		if len(data) != 5 {
			t.Fatal(data)
		}
		steps := data["steps"].([]any)
		if steps[0].(map[string]any)["id"] != "step_actual" {
			t.Fatal(data)
		}
		return data["approved"].(bool)
	}
	if approved() {
		t.Fatal("active plan inferred approval")
	}
	p.ApprovedContentHash = p.ContentHash()
	if !approved() {
		t.Fatal("valid approval missing")
	}
	p.Content = "Unapproved revision"
	if approved() {
		t.Fatal("stale approval accepted")
	}
}

func TestSnapshotAccountingAndCompactionExcludeHistoricalCopies(t *testing.T) {
	req := prepareAgentRequest(AgentRequest{RunID: "r", Context: testPack(model.ModeChat, model.ScopeAllPages, false, "hello"), ActiveSkills: []model.RunSkill{{ID: "story", Content: "instructions"}}})
	history := append(append([]llm.Message{}, req.RuntimeContext...), req.Messages...)
	req.Messages = llm.NormalizeHistory(history)
	req = prepareAgentRequest(req)
	messages := providerMessages(req)
	snapshot := (contextengine.PromptEstimator{}).Estimate(contextengine.PromptEstimateInput{Messages: messages})
	if snapshot.Buckets[contextengine.BucketRuntime] == 0 || len(req.RuntimeContext) != 8 {
		t.Fatal(snapshot)
	}
	for _, m := range req.Messages {
		if m.Metadata != nil && m.Metadata.Kind == "context" {
			t.Fatal("derived snapshot in history")
		}
	}
	// A second preparation is identical and cannot double the current snapshot.
	again := prepareAgentRequest(req)
	second := (contextengine.PromptEstimator{}).Estimate(contextengine.PromptEstimateInput{Messages: providerMessages(again)})
	if !reflect.DeepEqual(snapshot, second) {
		t.Fatal("snapshot counted twice")
	}
}

func TestRuntimeSnapshotRefreshesAfterToolsAndKeepsConversation(t *testing.T) {
	root, _, pack := generationPackFixture(t)
	transcript := &recordingTranscript{}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("read-manifest", "read_resource", map[string]any{"resource": "manifest"}),
		toolCall("edit-manifest", "edit_manifest", map[string]any{"goal": "Updated goal"}),
		finishCall("Updated the presentation goal."),
	}}
	steering := &scriptedSteering{batches: [][]SteeringInput{nil, {{ID: "feedback", Content: "Keep the slide content unchanged."}}}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{RunID: "snapshot-loop", ProjectDir: root, Context: pack, Transcript: transcript, Steering: steering, Prompter: autoApprovingResourcePrompter{projectDir: root}})
	if outcome.Status != StatusCompleted || len(agent.requests) != 3 {
		t.Fatalf("outcome=%+v requests=%d", outcome, len(agent.requests))
	}
	for _, req := range agent.requests {
		assertCurrentTaskContext(t, req)
		if len(req.RuntimeContext) != 8 {
			t.Fatal("missing runtime module")
		}
	}
	if len(agent.requests[0].ProjectState.Changes) != 0 {
		t.Fatal("first request invented changes")
	}
	if !reflect.DeepEqual(agent.requests[2].ProjectState.Changes, []ProjectChange{{"manifest", "updated"}}) {
		t.Fatal(agent.requests[2].ProjectState)
	}
	if len(agent.requests[2].Messages) < len(agent.requests[1].Messages) || !reflect.DeepEqual(agent.requests[2].Messages[:len(agent.requests[1].Messages)], agent.requests[1].Messages) {
		t.Fatal("tool loop rewrote earlier conversation")
	}
	for _, id := range []string{"read-manifest", "edit-manifest"} {
		call, result := toolRoundMessages(transcript.messages, id)
		if len(call.ToolCalls) != 1 || result.ToolCallID != id {
			t.Fatal("missing tool round", id)
		}
	}
	for _, message := range transcript.messages {
		if m := message.Metadata; m != nil && m.Kind == "context" {
			t.Fatal("persisted snapshot")
		}
	}
}
