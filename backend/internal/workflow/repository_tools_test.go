package workflow

import (
	"context"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type componentLoaderStub struct{ component model.Component }

func (s componentLoaderStub) Get(string) (model.Component, error) { return s.component, nil }

type skillLoaderStub struct{ skills []model.RunSkill }

func (s skillLoaderStub) ResolveDynamic([]string) ([]model.RunSkill, error) { return s.skills, nil }

func TestLoadComponentReturnsPrivateContentAndPublicResource(t *testing.T) {
	tool := loadComponentTool{loader: componentLoaderStub{component: model.Component{
		ID: "metric", Name: "Metric", HTML: "<div>secret reference</div>",
		LocalPath: "/private/component/index.html", OpenURL: "vscode://file/private/component/index.html",
	}}}
	result := tool.Execute(context.Background(), DomainToolInput{Args: map[string]any{"id": "metric"}})
	if !result.OK || len(result.LoadedResources) != 1 || result.LoadedResources[0].Name != "Metric" {
		t.Fatalf("result=%+v", result)
	}
	payload, ok := (ToolPublicProjector{}).Completed("run", "call", "load_component", map[string]any{"id": "metric"}, result)
	if !ok || len(payload.Resources) != 1 || payload.Resources[0].OpenURL == "" {
		t.Fatalf("payload=%+v", payload)
	}
	if payload.Display.Detail != "" {
		t.Fatalf("public detail leaked content: %q", payload.Display.Detail)
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
	checkpoint := checkpointToolResult(structToolCall("call", "load_skill"), result)
	if len(checkpoint.LoadedResources) != 1 {
		t.Fatal("checkpoint omitted loaded resources")
	}
}

func structToolCall(id, name string) llm.ToolCall {
	return llm.ToolCall{ID: id, Name: name}
}
