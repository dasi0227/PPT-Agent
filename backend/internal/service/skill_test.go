package service

import (
	"errors"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"os"
	"strings"
	"testing"
)

func TestSkillResolutionAndFileLimits(t *testing.T) {
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
	if _, err = svc.SetDisabled("one", true); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Resolve([]string{"one"}); err == nil {
		t.Fatal("disabled skill loaded")
	}
	path, _ := svc.resources.payloadPath("skill", "two")
	writeRepositoryFile(t, path, strings.Repeat("x", maxSkillFileBytes+1))
	v, err := svc.Get("two")
	if err != nil || v.ContentState != "invalid" {
		t.Fatalf("oversize=%+v err=%v", v, err)
	}
	if _, err = svc.Resolve([]string{"two"}); err == nil {
		t.Fatal("oversized skill loaded")
	}
	before, _ := os.ReadFile(path)
	if _, err = svc.UpdateMetadata("two", "Rename", "Description", nil); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("metadata rewrote oversized payload")
	}
}
