package workflow

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/testsupport"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestReferenceMessagePartsPreserveMixedOrderAndDOMComment(t *testing.T) {
	attachment := model.AttachmentReference{ID: "att_one", OriginalName: "brand.png", MediaType: "image/png"}
	selection := model.DOMSelection{SelectionID: "sel_one", MarkerNo: 3, Comment: "字号缩小", SlideID: "sli_one"}
	parts := referenceMessageParts("调整这两处", "pro_one", []model.AttachmentReference{attachment}, []model.DOMSelection{selection}, []model.ReferenceOrderItem{
		{Kind: "dom", RefID: "sel_one"}, {Kind: "image", RefID: "att_one"},
	})
	if len(parts) != 4 || !strings.Contains(parts[1].Text, "<selected_dom>") || !strings.Contains(parts[1].Text, "字号缩小") || parts[2].Type != "text" || parts[3].Type != "image" {
		t.Fatalf("parts = %#v", parts)
	}
}

func TestDOMReferencesSurviveRequestPreparationSteeringAndReplay(t *testing.T) {
	for _, instruction := range []string{"回答", ""} {
		t.Run("instruction="+instruction, func(t *testing.T) {
			pack := testPack(model.ModeChat, model.ScopeCurrentPage, false, instruction)
			pack.Command.DOMSelections = []model.DOMSelection{
				{SelectionID: "sel_one", MarkerNo: 1, Comment: "复述第一处", SlideID: "sli_1"},
				{SelectionID: "sel_three", MarkerNo: 3, Comment: "复述第三处", SlideID: "sli_1"},
			}
			pack.Command.Attachments = []model.AttachmentReference{{ID: "att_one", OriginalName: "brand.png", MediaType: "image/png"}}
			pack.Command.ReferenceOrder = []model.ReferenceOrderItem{
				{Kind: "dom", RefID: "sel_three"}, {Kind: "image", RefID: "att_one"}, {Kind: "dom", RefID: "sel_one"},
			}
			req := prepareAgentRequest(AgentRequest{RunID: "run_refs", Context: pack})
			var initial llm.Message
			for _, message := range req.Messages {
				if m := message.Metadata; m != nil && m.Origin == "user" && m.Kind == "instruction" {
					initial = message
				}
			}
			text := initial.Text()
			if !strings.Contains(text, "复述第三处") || !strings.Contains(text, "复述第一处") || strings.Index(text, "复述第三处") > strings.Index(text, "复述第一处") {
				t.Fatalf("initial instruction lost or reordered comments: %q", text)
			}
			offset := 0
			if instruction != "" {
				offset = 1
			}
			if len(initial.Content) != offset+4 || initial.Content[offset+2].ImageRef != "project:p1/attachment:att_one/original" {
				t.Fatalf("mixed references missing or reordered: %+v", initial.Content)
			}
			state := &RunState{runID: req.RunID, pack: pack, messages: req.Messages}
			steering := &scriptedSteering{batches: [][]SteeringInput{{{
				ID: "steer_refs", ProjectID: "p1", Content: instruction,
				DOMSelections: pack.Command.DOMSelections, Attachments: pack.Command.Attachments, ReferenceOrder: pack.Command.ReferenceOrder,
			}}}}
			if err := (&Runtime{}).appendSteering(context.Background(), RuntimeInput{}, state, steering); err != nil {
				t.Fatal(err)
			}
			last := state.messages[len(state.messages)-1]
			if last.Metadata.Kind != "steering" || !strings.Contains(last.Text(), "复述第一处") || !strings.Contains(last.Text(), "复述第三处") || len(steering.injected) != 1 {
				t.Fatalf("steering references missing: %+v", last)
			}
			dir := t.TempDir()
			store := contextengine.NewJournalTranscriptStore(testsupport.NewJournal(dir))
			if err := store.Replace(dir, "thread_refs", state.messages); err != nil {
				t.Fatal(err)
			}
			reloaded, err := store.Load(dir, "thread_refs")
			if err != nil || len(reloaded) != len(state.messages) {
				t.Fatalf("transcript replay failed: %+v, err=%v", reloaded, err)
			}
			if !reflect.DeepEqual(reloaded[0].Content, initial.Content) || !reflect.DeepEqual(reloaded[len(reloaded)-1].Content, last.Content) {
				t.Fatalf("transcript replay lost references: %+v, err=%v", reloaded, err)
			}
			req.Messages = reloaded
			replayed := prepareAgentRequest(req)
			count := 0
			for _, message := range replayed.Messages {
				if m := message.Metadata; m != nil && m.Origin == "user" && m.Kind == "instruction" && m.RunID == req.RunID {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("replay duplicated initial multipart instruction: count=%d", count)
			}
		})
	}
}
