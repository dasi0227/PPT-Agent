package model

import (
	"errors"
	"strings"
	"testing"
)

func validDOMSelection() DOMSelection {
	rect := CanvasRect{X: 10, Y: 20, Width: 300, Height: 120}
	return DOMSelection{
		SelectionID: "sel_one", MarkerNo: 1, Kind: DOMSelectionElement, SlideID: "sli_one",
		HTMLRevision: 3, HTMLHash: "sha256:abc", Canvas: CanvasSize{Width: 1920, Height: 1080}, Rect: rect, Status: DOMSelectionActive,
		DOMTargets: []DOMTarget{{
			TargetID: "target_one", Tag: "div", Rect: rect, TextSummary: "Quarterly plan",
			Fingerprint: DOMFingerprint{Tag: "div", SiblingIndex: 0, TextSummaryHash: "fnv1a:1"},
			OuterHTML:   "<div class=\"card\">Quarterly plan</div>", Attributes: map[string]string{"class": "card"},
			BoxModel: DOMBoxModel{Content: rect}, ComputedStyle: map[string]string{"display": "block"},
		}},
	}
}

func TestDOMSelectionValidationAndReferenceOrder(t *testing.T) {
	selection := validDOMSelection()
	if err := ValidateDOMSelections([]DOMSelection{selection}, nil, []ReferenceOrderItem{{Kind: "dom", RefID: selection.SelectionID}}); err != nil {
		t.Fatalf("valid selection rejected: %v", err)
	}
	if err := ValidateDOMSelections([]DOMSelection{selection}, nil, nil); !errors.Is(err, ErrReferenceOrderInvalid) {
		t.Fatalf("missing order error = %v", err)
	}
	selection.Comment = strings.Repeat("界", MaxDOMSelectionComment+1)
	if err := selection.Validate(); !errors.Is(err, ErrDOMSelectionCommentTooLong) {
		t.Fatalf("comment error = %v", err)
	}
}

func TestDOMSelectionRejectsUnsafeSnapshotData(t *testing.T) {
	cases := []func(*DOMSelection){
		func(value *DOMSelection) { value.DOMTargets[0].Attributes["onclick"] = "steal()" },
		func(value *DOMSelection) { value.DOMTargets[0].Attributes["src"] = "data:image/png;base64,secret" },
		func(value *DOMSelection) { value.DOMTargets[0].OuterHTML = "<div onload=\"steal()\"></div>" },
		func(value *DOMSelection) { value.Rect.X = -1 },
		func(value *DOMSelection) { value.DOMTargets[0].ComputedStyle["content"] = "secret" },
	}
	for index, mutate := range cases {
		value := validDOMSelection()
		mutate(&value)
		if err := value.Validate(); !errors.Is(err, ErrDOMSelectionInvalid) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}
}

func TestDOMSelectionEnforcesTargetAndCombinedByteLimits(t *testing.T) {
	selection := validDOMSelection()
	selection.DOMTargets = make([]DOMTarget, MaxDOMTargets+1)
	if err := selection.Validate(); !errors.Is(err, ErrDOMSelectionLimit) {
		t.Fatalf("target limit error = %v", err)
	}

	selection = validDOMSelection()
	selection.DOMTargets[0].OuterHTML = strings.Repeat("x", 16*1024+1)
	if err := selection.Validate(); !errors.Is(err, ErrDOMSelectionInvalid) {
		t.Fatalf("outer html limit error = %v", err)
	}
}

func TestDOMSelectionRequiresTextIntentEvenWhenAnImageIsAttached(t *testing.T) {
	selection := validDOMSelection()
	attachment := AttachmentReference{ID: "att_one", OriginalName: "reference.png", MediaType: "image/png", SizeBytes: 100, Width: 10, Height: 10}
	command := RunCommand{
		Scope: modelScopeForDOMTest(), Mode: ModeExecute, Attachments: []AttachmentReference{attachment}, DOMSelections: []DOMSelection{selection},
		ReferenceOrder: []ReferenceOrderItem{{Kind: "image", RefID: attachment.ID}, {Kind: "dom", RefID: selection.SelectionID}},
	}
	if err := command.Validate(); !errors.Is(err, ErrInvalidRunCommand) {
		t.Fatalf("image incorrectly substituted for DOM intent: %v", err)
	}
	command.DOMSelections[0].Comment = "缩小字号"
	if err := command.Validate(); err != nil {
		t.Fatalf("DOM comment should satisfy intent: %v", err)
	}
}

func modelScopeForDOMTest() RunScope {
	return NewRunScope(ScopeObjectPresentation, ScopeCurrentPage, "sli_one")
}
