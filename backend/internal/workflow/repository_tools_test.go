package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

type componentLoaderStub map[string]model.Component

func (s componentLoaderStub) Get(id string) (model.Component, error) {
	component, ok := s[id]
	if !ok {
		return model.Component{}, store.ErrResourceNotFound
	}
	return component, nil
}

type skillLoaderStub struct {
	skills []model.RunSkill
	err    error
}

func (s skillLoaderStub) ResolveDynamic([]string) ([]model.RunSkill, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.skills, nil
}

func TestLoadComponentsBatchPreservesSnapshotsAndVisibleContent(t *testing.T) {
	diagramHTML := strings.Repeat("PRIVATE_DIAGRAM", 32<<10)
	tool := loadComponentTool{loader: componentLoaderStub{
		"metric": {ResourceContentState: model.ResourceContentState{ContentState: "ready"},
			ID: "metric", Name: "Metric", HTML: "REPOSITORY_VERSION",
			LocalPath: "/private/metric.html", OpenURL: "vscode://file/private/metric.html"},
		"diagram": {ResourceContentState: model.ResourceContentState{ContentState: "ready"},
			ID: "diagram", Name: "Diagram", HTML: diagramHTML},
	}}
	pinned := model.RunComponent{ID: "metric", Name: "Metric", HTML: strings.Repeat("PINNED_VERSION", 32<<10)}
	input := DomainToolInput{Args: map[string]any{"ids": []any{"metric"}}, ActiveSkills: &ActiveSkillSet{}}
	input.Context.Command.Components = []model.RunComponent{pinned}
	first := tool.Execute(context.Background(), input)
	if !first.OK || !strings.Contains(first.Observation, pinned.HTML) || strings.Contains(first.Observation, "REPOSITORY_VERSION") {
		t.Fatalf("selected snapshot was not used: %+v", first)
	}
	input.Messages = appendBatchObservations(nil, []llm.ToolCall{{ID: "first", Name: "load_component"}}, "", []ToolResult{first})
	input.Args = map[string]any{"ids": []any{"metric", "diagram"}}
	result := tool.Execute(context.Background(), input)
	if !result.OK || len(result.LoadedResources) != 2 {
		t.Fatalf("result=%+v", result)
	}
	var observation struct {
		Loaded           int
		Components       []map[string]string
		AlreadyAvailable []string `json:"already_available"`
	}
	if err := json.Unmarshal([]byte(result.Observation), &observation); err != nil {
		t.Fatal(err)
	}
	if len(observation.Components) != 1 || observation.Components[0]["id"] != "diagram" ||
		!reflect.DeepEqual(observation.AlreadyAvailable, []string{"metric"}) || strings.Contains(result.Observation, pinned.HTML) {
		t.Fatalf("visible component was repeated or new content missing: %s", result.Observation)
	}
	_, snapshots := input.ActiveSkills.Snapshot()
	if len(snapshots) != 2 || snapshots[0].HTML != pinned.HTML || snapshots[1].HTML != diagramHTML {
		t.Fatalf("snapshots=%+v", snapshots)
	}
	if result.ObservationMetadata == nil || len(result.ObservationMetadata.Resources) != 1 || result.ObservationMetadata.Resources[0].Key != "component/diagram" {
		t.Fatalf("resource visibility metadata=%+v", result.ObservationMetadata)
	}
	payload, ok := (ToolPublicProjector{}).Completed("run", "call", "load_component", input.Args, result)
	if !ok || len(payload.Resources) != 2 || payload.Resources[0].OpenURL == "" || payload.Display.Label != "已加载 2 个组件" {
		t.Fatalf("payload=%+v", payload)
	}
	if payload.Display.Detail != "" {
		t.Fatalf("public detail leaked content: %q", payload.Display.Detail)
	}
	// A durable snapshot is not visible context after compaction: return both bodies again.
	input.Messages = nil
	reloaded := tool.Execute(context.Background(), input)
	if !reloaded.OK || !strings.Contains(reloaded.Observation, pinned.HTML) || !strings.Contains(reloaded.Observation, "PRIVATE_DIAGRAM") {
		t.Fatalf("bodies were not restored after compaction: %+v", reloaded)
	}
}

func TestLoadComponentsRejectsInvalidBatchesWithoutPartialState(t *testing.T) {
	tool := loadComponentTool{loader: componentLoaderStub{
		"ready":    {ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "ready", HTML: "reference"},
		"disabled": {ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "disabled", Disabled: true},
		"broken":   {ResourceContentState: model.ResourceContentState{ContentState: "invalid", ContentError: "body must be valid UTF-8"}, ID: "broken"},
		"no-body":  {ResourceContentState: model.ResourceContentState{ContentState: "missing"}, ID: "no-body"},
	}}
	for _, tc := range []struct {
		name     string
		args     map[string]any
		code     string
		failedID string
	}{
		{"old id", map[string]any{"id": "ready"}, CodeToolArgumentInvalid, ""},
		{"empty", map[string]any{"ids": []any{}}, CodeToolArgumentInvalid, ""},
		{"duplicate", map[string]any{"ids": []any{"ready", "ready"}}, CodeToolArgumentInvalid, ""},
		{"invalid id", map[string]any{"ids": []any{"ready", "../private"}}, CodeToolArgumentInvalid, ""},
		{"too many", map[string]any{"ids": []any{"a", "b", "c", "d", "e", "f", "g", "h", "i"}}, CodeToolArgumentInvalid, ""},
		{"missing", map[string]any{"ids": []any{"ready", "missing"}}, "COMPONENT_NOT_FOUND", "missing"},
		{"missing body", map[string]any{"ids": []any{"ready", "no-body"}}, "COMPONENT_NOT_FOUND", "no-body"},
		{"disabled", map[string]any{"ids": []any{"ready", "disabled"}}, model.CodeResourceNotDisclosed, "disabled"},
		{"broken", map[string]any{"ids": []any{"ready", "broken"}}, model.CodeInternal, "broken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := model.RunComponent{ID: "ready", HTML: "previous snapshot"}
			active := &ActiveSkillSet{Components: []model.RunComponent{previous}}
			result := tool.Execute(context.Background(), DomainToolInput{Args: tc.args, ActiveSkills: active})
			if result.OK || result.Code != tc.code || !strings.Contains(result.Summary, tc.failedID) {
				t.Fatalf("result=%+v", result)
			}
			_, snapshots := active.Snapshot()
			if !reflect.DeepEqual(snapshots, []model.RunComponent{previous}) || len(result.LoadedResources) != 0 || result.ObservationMetadata != nil {
				t.Fatalf("failed batch changed state or reported loaded resources: %+v %+v", snapshots, result)
			}
		})
	}
}

func TestLoadSkillDeduplicatesAcrossRunState(t *testing.T) {
	skill := model.RunSkill{ID: "story", Name: "Story", Description: "Structure", Content: strings.Repeat("Private instructions", 32<<10)}
	active := &ActiveSkillSet{Skills: []model.RunSkill{skill}}
	repositorySkill := skill
	repositorySkill.Content = "NEW_REPOSITORY_INSTRUCTIONS"
	tool := loadSkillTool{loader: skillLoaderStub{skills: []model.RunSkill{repositorySkill}}}
	input := DomainToolInput{Args: map[string]any{"ids": []any{"story"}}, ActiveSkills: active}
	input.Context.Command.Skills = []model.RunSkill{skill}
	result := tool.Execute(context.Background(), input)
	if !result.OK || len(active.Skills) != 1 || len(result.LoadedResources) != 1 {
		t.Fatalf("result=%+v active=%+v", result, active)
	}
	if !strings.Contains(result.Observation, skill.Content) || active.Skills[0].Content != skill.Content {
		t.Fatal("large selected skill snapshot was replaced or omitted")
	}
	input.Messages = appendBatchObservations(nil, []llm.ToolCall{{ID: "loaded", Name: "load_skill"}}, "", []ToolResult{result})
	again := tool.Execute(context.Background(), input)
	var view struct {
		Skills           []map[string]string `json:"skills"`
		AlreadyAvailable []string            `json:"already_available"`
	}
	if err := json.Unmarshal([]byte(again.Observation), &view); err != nil || len(view.Skills) != 0 || !reflect.DeepEqual(view.AlreadyAvailable, []string{"story"}) {
		t.Fatalf("visible skill body was not deduplicated: %s", again.Observation)
	}
	for _, failure := range []error{
		model.NewAgentError("SKILL_NOT_FOUND", "load_skill", errors.New("missing skill")),
		model.RepositoryDisabledError("skill", "story"),
		model.RepositoryContentError("skill", "story", model.ResourceContentState{ContentState: "invalid", ContentError: "invalid UTF-8"}),
		&fs.PathError{Op: "read", Path: "SKILL.md", Err: fs.ErrPermission},
	} {
		before := append([]model.RunSkill(nil), active.Skills...)
		result := (loadSkillTool{loader: skillLoaderStub{err: failure}}).Execute(context.Background(), input)
		if result.OK || !reflect.DeepEqual(active.Skills, before) || len(result.LoadedResources) != 0 || result.ObservationMetadata != nil {
			t.Fatalf("failed skill batch changed state: %+v", result)
		}
	}
}
