package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestSkillServiceListsOnlyValidSkillsAndResolvesSnapshots(t *testing.T) {
	root := t.TempDir()
	writeSkillFixture(t, root, "story", "演示叙事", "梳理页面叙事。", "Always lead with the conclusion.")
	writeSkillFixture(t, root, "visual", "视觉层级", "优化页面信息层级。", "Use contrast deliberately.")
	writeSkillFixture(t, root, "missing-description", "无描述", "", "Ignored.")

	service := NewSkillService(WorkRoot(root))
	skills, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 || skills[0].Name != "演示叙事" || skills[1].Name != "视觉层级" {
		t.Fatalf("unexpected skills: %+v", skills)
	}
	selected, err := service.Resolve([]string{"story", "visual"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].Content != "Always lead with the conclusion." ||
		selected[0].OpenURL == "" || selected[0].LocalPath == "" {
		t.Fatalf("unexpected selected skill snapshots: %+v", selected)
	}
}

func TestSkillServiceRejectsInvalidSelections(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"one", "two", "three", "four"} {
		writeSkillFixture(t, root, id, id, "description", "instructions")
	}
	service := NewSkillService(WorkRoot(root))

	for _, ids := range [][]string{
		{"one", "one"},
		{"missing"},
		{"one", "two", "three", "four"},
		{"../one"},
	} {
		_, err := service.Resolve(ids)
		var agentErr *model.AgentError
		if !errors.As(err, &agentErr) ||
			(agentErr.Code != "SKILL_SELECTION_INVALID" && agentErr.Code != "SKILL_NOT_FOUND") {
			t.Fatalf("selection %v returned %v", ids, err)
		}
	}
}

func writeSkillFixture(t *testing.T, root, id, name, description, body string) {
	t.Helper()
	dir := filepath.Join(root, "skills", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := "---\nname: " + name + "\ndescription: " + description + "\n---\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}
