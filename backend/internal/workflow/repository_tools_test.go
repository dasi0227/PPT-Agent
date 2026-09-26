package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type componentLoaderStub map[string]model.Component

func (s componentLoaderStub) Get(id string) (model.Component, error) {
	component, ok := s[id]
	if !ok {
		return model.Component{}, errors.New("not found")
	}
	return component, nil
}

type skillLoaderStub struct{ skills []model.RunSkill }

func (s skillLoaderStub) ResolveDynamic([]string) ([]model.RunSkill, error) { return s.skills, nil }

func TestLoadComponentsBatchPreservesSnapshotsAndVisibleContent(t *testing.T) {
	tool := loadComponentTool{loader: componentLoaderStub{
		"metric": {ResourceContentState: model.ResourceContentState{ContentState: "ready"},
			ID: "metric", Name: "Metric", HTML: "REPOSITORY_VERSION",
			LocalPath: "/private/metric.html", OpenURL: "vscode://file/private/metric.html"},
		"diagram": {ResourceContentState: model.ResourceContentState{ContentState: "ready"},
			ID: "diagram", Name: "Diagram", HTML: "PRIVATE_DIAGRAM"},
	}}
	pinned := model.RunComponent{ID: "metric", Name: "Metric", HTML: "PINNED_VERSION"}
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
	if observation.Loaded != 1 || len(observation.Components) != 1 || observation.Components[0]["id"] != "diagram" ||
		!reflect.DeepEqual(observation.AlreadyAvailable, []string{"metric"}) || strings.Contains(result.Observation, pinned.HTML) {
		t.Fatalf("visible component was repeated or new content missing: %s", result.Observation)
	}
	_, snapshots := input.ActiveSkills.Snapshot()
	if len(snapshots) != 2 || snapshots[0].HTML != pinned.HTML || snapshots[1].HTML != "PRIVATE_DIAGRAM" {
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
		"broken":   {ID: "broken"},
		"large":    {ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "large", HTML: strings.Repeat("x", maxLoadedComponentBytes)},
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
		{"missing", map[string]any{"ids": []any{"ready", "missing"}}, CodeResourceNotFound, "missing"},
		{"disabled", map[string]any{"ids": []any{"ready", "disabled"}}, CodeResourceNotFound, "disabled"},
		{"broken", map[string]any{"ids": []any{"ready", "broken"}}, CodeResourceNotFound, "broken"},
		{"budget", map[string]any{"ids": []any{"ready", "large"}}, CodeContentTooLarge, "large"},
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
	skill := model.RunSkill{ID: "story", Name: "Story", Description: "Structure", Content: "Private instructions"}
	active := &ActiveSkillSet{Skills: []model.RunSkill{skill}}
	result := (loadSkillTool{loader: skillLoaderStub{skills: []model.RunSkill{skill}}}).Execute(
		context.Background(),
		DomainToolInput{Args: map[string]any{"ids": []any{"story"}}, ActiveSkills: active},
	)
	if !result.OK || len(active.Skills) != 1 || len(result.LoadedResources) != 1 {
		t.Fatalf("result=%+v active=%+v", result, active)
	}
}
