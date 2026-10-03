package contextengine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/testsupport"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

func TestReviewRequirementsSurviveCompactionWithReferencesAndAnswers(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	journal := testsupport.NewJournal(dir)
	appendEvent := func(kind string, value any) {
		t.Helper()
		raw, _ := json.Marshal(value)
		if _, err := journal.AppendThreadEvent(ctx, "thread", threadjournal.Event{RunID: "run", Type: kind, Payload: raw}); err != nil {
			t.Fatal(err)
		}
	}
	appendEvent("run.accepted", map[string]any{"command": model.RunCommand{Instruction: "完整原始需求", Attachments: []model.AttachmentReference{{ID: "att_reference", OriginalName: "ref.png"}}}})
	appendEvent("steering.accepted", model.SteeringMessage{Content: "保留原始数据"})
	appendEvent(string(model.EventQuestionAsked), model.QuestionAskedPayload{QuestionID: "question", Questions: []model.QuestionField{{ID: "field", Question: "正文语言", Options: []model.QuestionOption{{ID: "zh", Label: "中文"}}}}})
	appendEvent(string(model.EventQuestionAnswered), model.QuestionAnsweredPayload{QuestionID: "question", Answer: model.QuestionAnswer{Answers: []model.QuestionFieldAnswer{{QuestionID: "field", SelectedOptionID: "zh"}}}})
	store := NewJournalTranscriptStore(journal)
	if err := store.Replace(dir, "thread", []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("lossy summary")}, {Role: llm.RoleAssistant, Content: llm.TextContent("PRIVATE_ASSISTANT")}}); err != nil {
		t.Fatal(err)
	}
	inputs, err := store.LoadReviewUserInputs(ctx, dir, "thread")
	if err != nil || len(inputs) != 3 || inputs[0].Text != "完整原始需求" || inputs[0].Attachments[0].ID != "att_reference" || inputs[1].Text != "保留原始数据" || !strings.Contains(inputs[2].Text, "正文语言") || !strings.Contains(inputs[2].Text, "中文") {
		t.Fatalf("incomplete user requirements: %v %v", inputs, err)
	}
	raw, _ := json.Marshal(inputs)
	if strings.Contains(string(raw), "PRIVATE_ASSISTANT") || strings.Contains(string(raw), "lossy summary") {
		t.Fatal("model transcript entered authoritative requirements")
	}
}
