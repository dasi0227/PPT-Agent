package model

import (
	"testing"
)

func TestRunCommandValidation(t *testing.T) {
	valid := RunCommand{
		Scope:       RunScope{Artifact: ArtifactPPT, Level: ScopeSlide, SlideID: "stable"},
		Mode:      ModeExecute,
		Instruction: "revise",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}
	cases := []RunCommand{
		{Scope: RunScope{Artifact: "generate", Level: ScopeDeck}, Mode: valid.Mode, Instruction: "x"},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: "current"}, Mode: valid.Mode, Instruction: "x"},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: ScopeSlide}, Mode: valid.Mode, Instruction: "x"},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: ScopeDeck, SlideID: "current"}, Mode: valid.Mode, Instruction: "x"},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: ScopeDeck, SlideID: "stable"}, Mode: valid.Mode, Instruction: "x"},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: ScopeDeck}, Mode: "consult", Instruction: "x"},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: ScopeDeck}, Mode: valid.Mode, Instruction: "  "},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: ScopeDeck}, Mode: valid.Mode, Instruction: "x", Options: RunOptions{Language: "fr-FR"}},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: ScopeDeck}, Mode: valid.Mode, Instruction: "x", Options: RunOptions{Range: "8-15"}},
		{Scope: RunScope{Artifact: ArtifactSpec, Level: ScopeSlide, SlideID: "stable"}, Mode: valid.Mode, Instruction: "x", Options: RunOptions{Range: SlideRangeFiveToEight}},
	}
	for i, command := range cases {
		if err := command.Validate(); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
}

func TestRunCommandValidationAcceptsPlanIntentAndOptions(t *testing.T) {
	command := RunCommand{
		Scope:       RunScope{Artifact: ArtifactSpec, Level: ScopeDeck},
		Mode:      ModePlan,
		Instruction: "plan the work",
		Options:     RunOptions{Language: LanguageChinese, Range: SlideRangeNineToFifteen},
	}
	if err := command.Validate(); err != nil {
		t.Fatalf("plan mode rejected: %v", err)
	}
}

func TestSlideRangeContains(t *testing.T) {
	cases := []struct {
		value SlideRange
		count int
		want  bool
	}{
		{SlideRangeFiveToEight, 5, true},
		{SlideRangeFiveToEight, 9, false},
		{SlideRangeNineToFifteen, 15, true},
		{SlideRangeSixteenToTwentyFive, 16, true},
		{SlideRangeSixteenToTwentyFive, 26, false},
		{SlideRangeTwentySixPlus, 26, true},
		{"", 10, false},
	}
	for _, tc := range cases {
		if got := tc.value.Contains(tc.count); got != tc.want {
			t.Errorf("%q.Contains(%d)=%v want %v", tc.value, tc.count, got, tc.want)
		}
	}
}
