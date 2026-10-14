package workflow

import (
	"context"
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
	skill model.RepositorySkill
	err   error
}

func (s skillLoaderStub) Get(id string) (model.RepositorySkill, error) {
	if s.err != nil {
		return model.RepositorySkill{}, s.err
	}
	if s.skill.ID != id {
		return model.RepositorySkill{}, store.ErrResourceNotFound
	}
	return s.skill, nil
}

func assertRepositoryLoadSuccess(t *testing.T, result ToolResult) {
	t.Helper()
	messages := appendBatchObservations(nil, []llm.ToolCall{{ID: "load"}}, "", []ToolResult{result})
	if !result.OK || !result.Applied || len(result.LoadedResources) != 1 || messages[1].Text() != `{"status":"success"}` ||
		len(messages[1].Content) != 1 || result.ObservationMetadata != nil {
		t.Fatalf("invalid load receipt: %+v", result)
	}
}

func TestLoadComponentPreservesSnapshotsAndActiveContent(t *testing.T) {
	diagramHTML := strings.Repeat("PRIVATE_DIAGRAM", 32<<10)
	tool := loadComponentTool{loader: componentLoaderStub{
		"metric": {ResourceContentState: model.ResourceContentState{ContentState: "ready"},
			ID: "metric", Name: "Metric", HTML: "REPOSITORY_VERSION"},
		"diagram": {ResourceContentState: model.ResourceContentState{ContentState: "ready"},
			ID: "diagram", Name: "Diagram", HTML: diagramHTML},
	}}
	pinned := model.RunComponent{ID: "metric", Name: "Metric", HTML: strings.Repeat("PINNED_VERSION", 32<<10),
		LocalPath: "/private/metric.html", OpenURL: "vscode://file/private/metric.html"}
	input := DomainToolInput{Args: map[string]any{"id": "metric"}, ActiveSkills: &ActiveSkillSet{Components: []model.RunComponent{pinned}}}
	input.Context.Command.Components = []model.RunComponent{pinned}
	first := tool.Execute(context.Background(), input)
	assertRepositoryLoadSuccess(t, first)
	payload, ok := (ToolPublicProjector{}).Completed("run", "call", "load_component", input.Args, first)
	if !ok || len(payload.Resources) != 1 || payload.Resources[0].ID != pinned.ID || payload.Resources[0].OpenURL != pinned.OpenURL || payload.Display.Label != "已加载 1 个组件" || payload.Display.Detail != "" {
		t.Fatalf("selected resource identity or public projection lost: %+v", payload)
	}
	input.Args = map[string]any{"id": "diagram"}
	result := tool.Execute(context.Background(), input)
	assertRepositoryLoadSuccess(t, result)
	_, snapshots := input.ActiveSkills.Snapshot()
	if len(snapshots) != 2 || snapshots[0].HTML != pinned.HTML || snapshots[1].HTML != diagramHTML {
		t.Fatal("selected snapshot changed or large loaded body was truncated")
	}
	// Compaction removes history, but complete active bodies remain available.
	input.Messages = nil
	assertRepositoryLoadSuccess(t, tool.Execute(context.Background(), input))
	// A past receipt alone does not activate a body in a new run.
	input.Messages = appendBatchObservations(nil, []llm.ToolCall{{ID: "old", Name: "load_component", Args: input.Args}}, "", []ToolResult{result})
	input.ActiveSkills = &ActiveSkillSet{}
	assertRepositoryLoadSuccess(t, tool.Execute(context.Background(), input))
}

func TestLoadComponentRejectsInvalidInputWithoutChangingState(t *testing.T) {
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
		{"old ids", map[string]any{"ids": []any{"ready"}}, CodeToolArgumentInvalid, ""},
		{"missing id", map[string]any{}, CodeToolArgumentInvalid, ""},
		{"empty id", map[string]any{"id": ""}, CodeToolArgumentInvalid, ""},
		{"array id", map[string]any{"id": []any{"ready"}}, CodeToolArgumentInvalid, ""},
		{"invalid id", map[string]any{"id": "../private"}, CodeToolArgumentInvalid, ""},
		{"missing", map[string]any{"id": "missing"}, model.CodeNotFound, "missing"},
		{"missing body", map[string]any{"id": "no-body"}, model.CodeNotFound, "no-body"},
		{"disabled", map[string]any{"id": "disabled"}, model.CodeResourceNotDisclosed, "disabled"},
		{"broken", map[string]any{"id": "broken"}, model.CodeInternal, "broken"},
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
				t.Fatal("failed load changed state or reported loaded resources")
			}
		})
	}
}

func TestLoadSkillDeduplicatesAcrossRunState(t *testing.T) {
	skill := model.RunSkill{ID: "story", Name: "Story", Description: "Structure", Content: strings.Repeat("Private instructions", 32<<10)}
	active := &ActiveSkillSet{Skills: []model.RunSkill{skill}}
	repositorySkill := model.RepositorySkill{ResourceContentState: model.ResourceContentState{ContentState: "ready"},
		ID: skill.ID, Name: skill.Name, Content: "NEW_REPOSITORY_INSTRUCTIONS"}
	tool := loadSkillTool{loader: skillLoaderStub{skill: repositorySkill}}
	input := DomainToolInput{Args: map[string]any{"id": "story"}, ActiveSkills: active}
	input.Context.Command.Skills = []model.RunSkill{skill}
	result := tool.Execute(context.Background(), input)
	assertRepositoryLoadSuccess(t, result)
	if len(active.Skills) != 1 || active.Skills[0].Content != skill.Content {
		t.Fatal("large selected skill snapshot was replaced or omitted")
	}
	input.Messages = appendBatchObservations(nil, []llm.ToolCall{{ID: "loaded", Name: "load_skill", Args: input.Args}}, "", []ToolResult{result})
	assertRepositoryLoadSuccess(t, tool.Execute(context.Background(), input))
	// Without an explicit selection, the current repository body replaces the active version.
	input.Context.Command.Skills = nil
	assertRepositoryLoadSuccess(t, tool.Execute(context.Background(), input))
	if active.Skills[0].Content != repositorySkill.Content {
		t.Fatal("changed repository body was not activated")
	}
	for _, args := range []map[string]any{{"ids": []any{"story"}}, {"id": ""}, {"id": "../story"}, {"id": []any{"story"}}} {
		invalid := input
		invalid.Args = args
		if result := tool.Execute(context.Background(), invalid); result.OK || result.Code != CodeToolArgumentInvalid {
			t.Fatalf("invalid single skill input was accepted: %+v", result)
		}
	}
	for _, tc := range []struct {
		loader skillLoaderStub
		code   string
	}{
		{skillLoaderStub{err: store.ErrResourceNotFound}, model.CodeNotFound},
		{skillLoaderStub{err: fs.ErrNotExist}, model.CodeNotFound},
		{skillLoaderStub{skill: model.RepositorySkill{ID: "story", Disabled: true}}, model.CodeResourceNotDisclosed},
		{skillLoaderStub{skill: model.RepositorySkill{ID: "story", ResourceContentState: model.ResourceContentState{ContentState: "missing"}}}, model.CodeNotFound},
		{skillLoaderStub{skill: model.RepositorySkill{ID: "story", ResourceContentState: model.ResourceContentState{ContentState: "invalid"}}}, model.CodeInternal},
		{skillLoaderStub{err: errors.New("repository unavailable")}, model.CodeInternal},
		{skillLoaderStub{err: &fs.PathError{Op: "read", Path: "SKILL.md", Err: fs.ErrPermission}}, model.CodeInternal},
	} {
		before := append([]model.RunSkill(nil), active.Skills...)
		result := (loadSkillTool{loader: tc.loader}).Execute(context.Background(), input)
		if result.OK || result.Code != tc.code || !reflect.DeepEqual(active.Skills, before) || len(result.LoadedResources) != 0 || result.ObservationMetadata != nil {
			t.Fatalf("failed skill load changed state or lost error classification: %+v", result)
		}
	}
}
