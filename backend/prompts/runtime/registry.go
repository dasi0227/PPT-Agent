package runtimeprompts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

//go:embed core/*.md modes/*.md playbooks/*.md guides/*.md rubrics/*.md business/*.md resources/*.md
var promptFiles embed.FS

type Module struct {
	ID      string
	Version string
	Path    string
	Body    string
	Hash    string
}

const Version = "2026-08-06.v1"

var modulePaths = map[string]string{
	"core_runtime_policy":              "core/core_runtime_policy.md",
	"mode_policy_talk":                 "modes/talk.md",
	"mode_policy_ask":                  "modes/ask.md",
	"mode_policy_plan":                 "modes/plan.md",
	"mode_policy_execute_direct":       "modes/execute_direct.md",
	"mode_policy_execute_planned":      "modes/execute_planned.md",
	"playbook_read_only_planning":      "playbooks/read_only_planning.md",
	"playbook_read_only_collaboration": "playbooks/read_only_collaboration.md",
	"playbook_spec_edit":               "playbooks/spec_edit.md",
	"playbook_slide_presentation_edit": "playbooks/slide_presentation_edit.md",
	"playbook_empty_deck_generation":   "playbooks/empty_deck_generation.md",
	"playbook_deck_coordinated_edit":   "playbooks/deck_coordinated_edit.md",
	"playbook_default":                 "playbooks/default.md",
	"completion_repair_guide":          "guides/completion_repair_guide.md",
	"finish_contract":                  "guides/finish_contract.md",
	"ppt_quality_rubric":               "rubrics/ppt_quality_rubric.md",
	"ppt_business_policy":              "business/ppt_business_policy.md",
	"resource_contracts":               "resources/resource_contracts.md",
}

func Load(id string) (Module, error) {
	rel, ok := modulePaths[id]
	if !ok {
		return Module{}, fmt.Errorf("runtime prompt module %q is not registered", id)
	}
	raw, err := promptFiles.ReadFile(path.Clean(rel))
	if err != nil {
		return Module{}, fmt.Errorf("runtime prompt module %q cannot be read: %w", id, err)
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return Module{}, fmt.Errorf("runtime prompt module %q is empty", id)
	}
	sum := sha256.Sum256([]byte(body))
	return Module{
		ID: id, Version: Version, Path: rel, Body: body,
		Hash: hex.EncodeToString(sum[:]),
	}, nil
}

func MustLoad(id string) Module {
	module, err := Load(id)
	if err != nil {
		panic(err)
	}
	return module
}
