package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestSkillResolutionKeepsSelectionConstraintsAndLoadsLargeBodies(t *testing.T) {
	root := t.TempDir()
	st := newMemoryResourceStore()
	svc := NewSkillService(WorkRoot(root), st)
	for _, id := range []string{"one", "two", "three", "four"} {
		registerFixture(t, root, st, "skill", id, id, "Description", "# Pure instructions\n")
	}
	selected, err := svc.Resolve([]string{"two", "one"})
	if err != nil || len(selected) != 2 || selected[0].ID != "two" || selected[0].Content != "# Pure instructions\n" {
		t.Fatalf("selected=%+v err=%v", selected, err)
	}
	for _, ids := range [][]string{{"one", "one"}, {"missing"}, {"one", "two", "three", "four"}, {"../one"}} {
		_, err := svc.Resolve(ids)
		var agentErr *model.AgentError
		if !errors.As(err, &agentErr) {
			t.Fatalf("selection %v: %v", ids, err)
		}
	}
	disabled := true
	if _, err = svc.resources.Patch(context.Background(), "skill", "one", ResourcePatch{Disabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Resolve([]string{"one"}); err == nil {
		t.Fatal("disabled skill loaded")
	}
	path, _ := svc.resources.payloadPath("skill", "two")
	large := strings.Repeat("x", 512<<10)
	writeRepositoryFile(t, path, large)
	v, err := svc.Get("two")
	if err != nil || v.ContentState != "ready" || v.Content != large {
		t.Fatalf("large skill body was rejected or truncated: state=%s err=%v", v.ContentState, err)
	}
	if skills, err := svc.Resolve([]string{"two", "three"}); err != nil || len(skills) != 2 || skills[0].Content != large {
		t.Fatalf("large skill selection was limited: %v", err)
	}
	before, _ := os.ReadFile(path)
	name, description := "Rename", "Description"
	tags := []string{}
	if _, err = svc.resources.Patch(context.Background(), "skill", "two", ResourcePatch{Name: &name, Description: &description, Tags: &tags}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("metadata rewrote large payload")
	}
	missingPath, _ := svc.resources.payloadPath("skill", "three")
	if err := os.Remove(missingPath); err != nil {
		t.Fatal(err)
	}
	if skills, err := svc.Resolve([]string{"two", "three"}); err == nil || len(skills) != 0 {
		t.Fatalf("missing body loaded part of the skill selection: %v", err)
	} else {
		var failure *model.AgentError
		if !errors.As(err, &failure) || failure.Code != "SKILL_NOT_FOUND" {
			t.Fatalf("missing skill body: %v", err)
		}
	}
	writeRepositoryFile(t, missingPath, "")
	if _, err := svc.Resolve([]string{"three"}); err == nil {
		t.Fatal("invalid skill body loaded")
	} else {
		var failure *model.AgentError
		if !errors.As(err, &failure) || failure.Code != model.CodeInternal {
			t.Fatalf("invalid skill body was treated as missing: %v", err)
		}
	}
	registerFixture(t, root, st, "skill", "new-large", "New large", "Description", large)
	if skills, err := svc.Resolve([]string{"new-large"}); err != nil || len(skills) != 1 || skills[0].Content != large {
		t.Fatalf("large skill registration was limited: %v", err)
	}
}
