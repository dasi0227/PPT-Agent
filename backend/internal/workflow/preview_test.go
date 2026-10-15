package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func validPreviewResponse() decision.Response {
	out := decision.Response{Model: "preview-test", Answers: map[string]decision.Answer{}}
	for _, rubric := range model.PreviewRubrics() {
		out.Answers[rubric.Dimension] = decision.ScoreAnswer{Type: "score", Score: 2.5}
	}
	out.Answers["mismatch"] = decision.ChoiceAnswer{Type: "choice", Choice: "spec_mismatch"}
	out.Answers["issue"] = decision.ChoiceAnswer{Type: "choice", Choice: "layout_issue"}
	return out
}
func TestRenderPreviewOptInCompleteMaterialsCachingAndInvalidation(t *testing.T) {
	dir, css, pack := generationPackFixture(t)
	html := strings.Replace(generationHTML, "<body>", "<head><style>h1{color:red}</style></head><body>", 1)
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(html))
	renderer := &cacheCountingRenderer{}
	tool := slideRenderTool{renderer: renderer, themes: staticThemeLoader{theme: model.Theme{ID: "clean", CSS: css, ResourceContentState: model.ResourceContentState{ContentState: "ready"}}}}
	count := 0
	missingLabel := ""
	provider := decisionFunc(func(_ context.Context, req decision.Request) (decision.Response, error) {
		count++
		if len(req.Input) != 6 || len(req.Questions) != 5 {
			t.Fatal("materials or questions missing")
		}
		for i, label := range []string{"QUALITY", "MANIFEST", "DESIGN", "SPEC", "HTML"} {
			if !strings.HasPrefix(req.Input[i].Text, label) {
				t.Fatalf("material %d out of order", i)
			}
			if label == missingLabel && req.Input[i].Text != label+"\n" {
				t.Fatalf("missing %s was not supplied as empty content", label)
			}
		}
		if req.Input[5].Image == nil || len(req.Input[5].Image.Data) == 0 || !strings.Contains(req.Input[4].Text, "<style>") {
			t.Fatal("full source or screenshot lost")
		}
		return validPreviewResponse(), nil
	})
	input := DomainToolInput{RunID: "preview_run", CallID: "render_1", ProjectDir: dir, Context: pack, Scope: pack.Command.Scope, Args: map[string]any{"slide_id": generationSlide, "need_preview": false}, Preview: decision.Snapshot{Identity: "one", Provider: provider}}
	result := tool.Execute(context.Background(), input)
	if !result.OK || result.Preview != nil || count != 0 {
		t.Fatal("false requested a preview")
	}
	input.Args["need_preview"] = true
	result = tool.Execute(context.Background(), input)
	if !result.OK || result.Preview == nil || count != 1 || renderer.calls != 1 {
		t.Fatalf("cached render did not assess: %+v count=%d", result, count)
	}
	var visible map[string]any
	if json.Unmarshal([]byte(result.ObservationParts[0].Text), &visible) != nil || visible["status"] != "success" || visible["slide_id"] != nil {
		t.Fatal("invalid observation")
	}
	preview := visible["preview"].(map[string]any)
	if len(preview) != 2 || preview["scores"] == nil || preview["judges"] == nil {
		t.Fatal("internal metadata exposed")
	}
	original := result.Preview
	input.CallID = "render_2"
	result = tool.Execute(context.Background(), input)
	if result.Preview == nil || count != 1 {
		t.Fatal("valid assessment was not reused")
	}
	input.Args["need_preview"] = false
	result = tool.Execute(context.Background(), input)
	if result.Preview != nil || count != 1 || strings.Contains(result.ObservationParts[0].Text, `"preview"`) {
		t.Fatal("false returned cached assessment")
	}
	design := *pack.Design.Design
	design.Requirements = []string{"Change the visual requirement"}
	raw, _ := json.Marshal(design)
	writeGenerationFile(t, dir, ".design.json", raw)
	input.Args["need_preview"] = true
	input.CallID = "render_3"
	result = tool.Execute(context.Background(), input)
	if !result.OK || result.Preview == nil || count != 2 || renderer.calls != 1 {
		t.Fatal("design changed without invalidating assessment independently of pixels")
	}
	current := currentPreview(context.Background(), dir, pack, *original)
	if current.Status != "stale" || current.Result != nil {
		t.Fatal("history still exposes stale scores")
	}
	runtime := NewRuntime(nil)
	runtime.Preview = input.Preview
	state := &RunState{pack: pack, reviewInstructions: []ReviewInstruction{{Text: "New user requirement", Attachments: []model.AttachmentReference{{ID: "att_missing"}}}}}
	runtime.invalidatePreviews(RuntimeInput{ProjectDir: dir}, state)
	input.CallID = "render_4"
	result = tool.Execute(context.Background(), input)
	if !result.OK || result.Preview == nil || count != 2 {
		t.Fatal("user requirements or attachments invalidated the resource-only assessment")
	}
	for _, resource := range []struct{ path, label string }{{".manifest.json", "MANIFEST"}, {".design.json", "DESIGN"}, {model.SlideSpecPath(generationSlide), "SPEC"}} {
		raw, err := os.ReadFile(filepath.Join(dir, resource.path))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(dir, resource.path)); err != nil {
			t.Fatal(err)
		}
		missingLabel = resource.label
		input.CallID = "missing_" + resource.label
		before := count
		result = tool.Execute(context.Background(), input)
		if !result.OK || result.Preview == nil || count != before+1 {
			t.Fatalf("missing %s blocked assessment: %+v", resource.label, result)
		}
		writeGenerationFile(t, dir, resource.path, raw)
		if current := currentPreview(context.Background(), dir, pack, *result.Preview); current.Status != "stale" {
			t.Fatalf("restored %s did not invalidate the empty-source assessment", resource.label)
		}
	}
	missingLabel = ""
	input.Preview.Identity = "two"
	runtime.Preview = input.Preview
	runtime.invalidatePreviews(RuntimeInput{ProjectDir: dir}, state)
	input.CallID = "changed_provider"
	before := count
	result = tool.Execute(context.Background(), input)
	if !result.OK || result.Preview == nil || count != before+1 {
		t.Fatal("provider change reused assessment")
	}
}

func TestRenderPreviewFailurePartialAndCancellation(t *testing.T) {
	for _, mode := range []string{"offline", "partial", "cancel", "material_changed"} {
		t.Run(mode, func(t *testing.T) {
			dir, css, pack := generationPackFixture(t)
			tool := slideRenderTool{renderer: reviewPNGRenderer{}, themes: staticThemeLoader{theme: model.Theme{ID: "clean", CSS: css, ResourceContentState: model.ResourceContentState{ContentState: "ready"}}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			provider := decisionFunc(func(context.Context, decision.Request) (decision.Response, error) {
				switch mode {
				case "offline":
					return decision.Response{}, errors.New("offline")
				case "partial":
					out := validPreviewResponse()
					delete(out.Answers, "issue")
					return out, nil
				case "cancel":
					cancel()
					return decision.Response{}, context.Canceled
				case "material_changed":
					writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(generationHTML+"<!-- changed -->"))
				}
				return validPreviewResponse(), nil
			})
			result := tool.Execute(ctx, DomainToolInput{RunID: "preview_run", CallID: "render", ProjectDir: dir, Context: pack, Scope: pack.Command.Scope, Args: map[string]any{"slide_id": generationSlide, "need_preview": true}, Preview: decision.Snapshot{Identity: "one", Provider: provider}})
			if result.Preview != nil || strings.Contains(modelToolObservation(result), `"preview"`) {
				t.Fatal("invalid assessment exposed")
			}
			switch mode {
			case "cancel":
				if result.OK || result.Code != CodeCanceled {
					t.Fatal("run cancellation swallowed")
				}
			case "material_changed":
				if result.OK || result.Code != model.CodeContentConflict {
					t.Fatal("render conflict swallowed")
				}
			default:
				if !result.OK {
					t.Fatal("assessment failure changed render success")
				}
			}
		})
	}
}

func TestPreviewRejectsAbsentOptInBeforeRendering(t *testing.T) {
	result := (slideRenderTool{}).Execute(context.Background(), DomainToolInput{Args: map[string]any{"slide_id": generationSlide}})
	if result.OK || result.Code != model.CodeToolArgumentInvalid {
		t.Fatalf("missing need_preview accepted: %+v", result)
	}
}
