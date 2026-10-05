package model

import (
	"fmt"
	"testing"
)

func TestRunCommandValidation(t *testing.T) {
	valid := RunCommand{
		Scope:       NewRunScope(ScopeCurrentPage, "sli_stable"),
		Mode:        ModeExecute,
		Instruction: "revise",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}
	cases := []RunCommand{
		{Scope: NewRunScope("generate"), Mode: valid.Mode, Instruction: "x"},
		{Scope: NewRunScope("current"), Mode: valid.Mode, Instruction: "x"},
		{Scope: NewRunScope(ScopeCurrentPage), Mode: valid.Mode, Instruction: "x"},
		{Scope: NewRunScope(ScopeCurrentPage, "current"), Mode: valid.Mode, Instruction: "x"},
		{Scope: NewRunScope(ScopeCustomPages, "stable"), Mode: valid.Mode, Instruction: "x"},
		{Scope: NewRunScope(ScopeAllPages), Mode: "consult", Instruction: "x"},
		{Scope: NewRunScope(ScopeAllPages), Mode: valid.Mode, Instruction: "  "},
	}
	for i, command := range cases {
		if err := command.Validate(); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
}

func TestRunCommandValidationForMentionedPages(t *testing.T) {
	valid := RunCommand{
		Scope: NewRunScope(ScopeAllPages), Mode: ModeExecute, Instruction: "sync",
		MentionedPages: []MentionedPage{{
			Kind: "slide", SlideID: "sli_a-1", Ordinal: 2, Title: "融资历程",
			SpecState: "ready", HTMLState: "available",
		}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid mentioned page rejected: %v", err)
	}
	cases := []RunCommand{
		{Scope: valid.Scope, Mode: valid.Mode, Instruction: "x", MentionedPages: []MentionedPage{{Kind: "section", SlideID: "sli_a"}}},
		{Scope: valid.Scope, Mode: valid.Mode, Instruction: "x", MentionedPages: []MentionedPage{{Kind: "slide", SlideID: "current"}}},
		{Scope: valid.Scope, Mode: valid.Mode, Instruction: "x", MentionedPages: []MentionedPage{{Kind: "slide", SlideID: "sli_a"}, {Kind: "slide", SlideID: "sli_a"}}},
	}
	tooMany := valid
	tooMany.MentionedPages = make([]MentionedPage, MaxMentionedPages+1)
	for i := range tooMany.MentionedPages {
		tooMany.MentionedPages[i] = MentionedPage{Kind: "slide", SlideID: fmt.Sprintf("sli_%d", i)}
	}
	cases = append(cases, tooMany)
	for i, command := range cases {
		if err := command.Validate(); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
}

func TestRunCommandValidationAcceptsPlanIntent(t *testing.T) {
	command := RunCommand{
		Scope:       NewRunScope(ScopeAllPages),
		Mode:        ModePlan,
		Instruction: "plan a Chinese deck with 9 to 15 pages",
	}
	if err := command.Validate(); err != nil {
		t.Fatalf("plan mode rejected: %v", err)
	}
}

func TestRunCommandValidationAcceptsAtMostThreeCompleteUniqueSkills(t *testing.T) {
	command := RunCommand{
		Scope: NewRunScope(ScopeAllPages),
		Mode:  ModeExecute, Instruction: "build",
		Skills: []RunSkill{
			{ID: "one", Name: "One", Description: "First", Content: "Use one."},
			{ID: "two", Name: "Two", Description: "Second", Content: "Use two."},
			{ID: "three", Name: "Three", Description: "Third", Content: "Use three."},
		},
	}
	if err := command.Validate(); err != nil {
		t.Fatalf("valid skills rejected: %v", err)
	}
	command.Skills = append(command.Skills, RunSkill{ID: "four", Name: "Four", Description: "Fourth", Content: "Use four."})
	if err := command.Validate(); err == nil {
		t.Fatal("four selected skills should fail")
	}
	command.Skills = []RunSkill{
		{ID: "one", Name: "One", Description: "First", Content: "Use one."},
		{ID: "one", Name: "Duplicate", Description: "Duplicate", Content: "Duplicate."},
	}
	if err := command.Validate(); err == nil {
		t.Fatal("duplicate selected skills should fail")
	}
}

func TestRunCommandValidationRequiresAtMostEightCompleteUniqueComponentNames(t *testing.T) {
	command := RunCommand{
		Scope: NewRunScope(ScopeAllPages),
		Mode:  ModeExecute, Instruction: "build",
	}
	for index := 0; index < MaxRunComponents; index++ {
		command.Components = append(command.Components, RunComponent{
			ID: "component", Name: fmt.Sprintf("Component %d", index), HTML: "<div>reference</div>",
		})
	}
	if err := command.Validate(); err != nil {
		t.Fatalf("valid components rejected: %v", err)
	}
	command.Components = append(command.Components, RunComponent{Name: "Ninth", HTML: "<div />"})
	if err := command.Validate(); err == nil {
		t.Fatal("nine referenced components should fail")
	}
	command.Components = []RunComponent{{Name: "Card", HTML: "<div />"}, {Name: " Card ", HTML: "<section />"}}
	if err := command.Validate(); err == nil {
		t.Fatal("duplicate trimmed component names should fail")
	}
	for _, component := range []RunComponent{{Name: "", HTML: "<div />"}, {Name: "Card", HTML: " "}} {
		command.Components = []RunComponent{component}
		if err := command.Validate(); err == nil {
			t.Fatalf("incomplete component should fail: %+v", component)
		}
	}
}
