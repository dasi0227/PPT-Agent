package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type recordedEvent struct {
	kind    model.EventType
	payload any
}

type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *eventRecorder) Emit(kind model.EventType, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{kind: kind, payload: payload})
}

func (r *eventRecorder) count(kind model.EventType) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, event := range r.events {
		if event.kind == kind {
			count++
		}
	}
	return count
}

type executorFunc func(context.Context, StepInput) (StepResult, error)

func (fn executorFunc) Execute(ctx context.Context, input StepInput) (StepResult, error) {
	return fn(ctx, input)
}

type countingPlanModel struct{ calls int }

func (m *countingPlanModel) Plan(_ contextengine.ContextPack, fallback WorkflowPlan) (WorkflowPlan, error) {
	m.calls++
	return fallback, nil
}

func TestExecutionStrategyRouterBaseline(t *testing.T) {
	cases := []struct {
		name         string
		artifact     model.Artifact
		level        model.TargetLevel
		intent       model.InteractionIntent
		instruction  string
		materialized model.MaterializationState
		want         ExecutionStrategy
	}{
		{"consult", model.ArtifactPresentation, model.TargetDeck, model.IntentConsult, "回顾当前状态", model.MaterializationFresh, StrategyRespond},
		{"blueprint field", model.ArtifactBlueprint, model.TargetSlide, model.IntentApply, "修改标题为增长战略", model.MaterializationFresh, StrategyDirectAction},
		{"html patch", model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "修改文本为增长战略", model.MaterializationFresh, StrategyDirectAction},
		{"first materialization", model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "生成当前页", model.MaterializationNotMaterialized, StrategyCompactWorkflow},
		{"single slide rebuild", model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "重建当前页结构", model.MaterializationFresh, StrategyCompactWorkflow},
		{"blueprint deck", model.ArtifactBlueprint, model.TargetDeck, model.IntentApply, "完善叙事", model.MaterializationFresh, StrategyFullPEV},
		{"presentation deck", model.ArtifactPresentation, model.TargetDeck, model.IntentApply, "统一风格", model.MaterializationFresh, StrategyFullPEV},
		{"multi slide", model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "批量修改多页标题", model.MaterializationFresh, StrategyFullPEV},
		{"add slide", model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "新增一页结论", model.MaterializationFresh, StrategyFullPEV},
		{"delete slide", model.ArtifactBlueprint, model.TargetSlide, model.IntentApply, "删除这一页", model.MaterializationFresh, StrategyFullPEV},
		{"reorder", model.ArtifactBlueprint, model.TargetSlide, model.IntentApply, "重排页面顺序", model.MaterializationFresh, StrategyFullPEV},
	}
	router := ExecutionStrategyRouter{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pack := testPack(tc.artifact, tc.level, tc.intent, tc.instruction, tc.materialized)
			if got := router.Decide(pack).Strategy; got != tc.want {
				t.Fatalf("strategy=%s want=%s", got, tc.want)
			}
		})
	}
}

func TestRespondHasNoPlanWritesOrRevision(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(model.ArtifactBlueprint, model.TargetSlide, model.IntentConsult, "解释当前页", model.MaterializationFresh)
	writePackArtifacts(t, dir, pack)
	events := &eventRecorder{}
	var disclosed []ToolDescriptor
	planSteps := -1
	executor := executorFunc(func(_ context.Context, input StepInput) (StepResult, error) {
		disclosed = append(disclosed, input.Selection.Descriptors...)
		planSteps = len(input.State.Plan.Steps)
		return StepResult{Summary: "只读答复", Artifacts: []ArtifactRef{}, Issues: []Issue{}}, nil
	})
	runtime := NewAdaptiveRuntime(executor)
	planner := &countingPlanModel{}
	runtime.Workflow.Planner.Model = planner
	before := readFile(t, filepath.Join(dir, model.SlideJSONPath("s1")))
	outcome := runtime.Run(context.Background(), RuntimeInput{
		RunID: "respond-run", ProjectDir: dir, Context: pack, Emitter: events,
	})
	after := readFile(t, filepath.Join(dir, model.SlideJSONPath("s1")))
	if outcome.Status != StatusCompleted || outcome.Strategy != StrategyRespond {
		t.Fatalf("outcome=%+v", outcome)
	}
	if planner.calls != 0 || events.count(model.EventPlanCreated) != 0 {
		t.Fatalf("respond invoked planner or emitted plan")
	}
	if planSteps != 0 {
		t.Fatalf("respond carried a synthetic plan with %d steps", planSteps)
	}
	if events.count(model.EventArtifactStaged) != 0 || events.count(model.EventArtifactCommitted) != 0 {
		t.Fatal("respond emitted write events")
	}
	if string(before) != string(after) {
		t.Fatal("respond modified an artifact")
	}
	for _, descriptor := range disclosed {
		if descriptor.Risk != RiskRead && descriptor.Risk != RiskControl {
			t.Fatalf("respond disclosed %s risk tool %s", descriptor.Risk, descriptor.Name)
		}
		for _, capability := range descriptor.Capabilities {
			if IsWriteCapability(capability) {
				t.Fatalf("respond disclosed write capability %s", capability)
			}
		}
	}
}

func TestDirectActionSkipsPlannerVerifiesAndCommits(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(model.ArtifactBlueprint, model.TargetSlide, model.IntentApply, "修改标题为：增长战略", model.MaterializationFresh)
	writePackArtifacts(t, dir, pack)
	events := &eventRecorder{}
	planner := &countingPlanModel{}
	commits := 0
	runtime := NewAdaptiveRuntime(CognitiveExecutor{})
	runtime.Workflow.Planner.Model = planner
	outcome := runtime.Run(context.Background(), RuntimeInput{
		RunID: "direct-run", ProjectDir: dir, Context: pack, Emitter: events,
		CommitMetadata: func(context.Context, ChangeSet) error { commits++; return nil },
	})
	if outcome.Status != StatusCompleted || outcome.Strategy != StrategyDirectAction {
		t.Fatalf("outcome=%+v", outcome)
	}
	if planner.calls != 0 || events.count(model.EventPlanCreated) != 0 {
		t.Fatal("direct action invoked planner or emitted plan")
	}
	if commits != 1 || events.count(model.EventVerificationCompleted) == 0 || events.count(model.EventArtifactCommitted) != 1 {
		t.Fatalf("commits=%d verification=%d artifact.committed=%d", commits, events.count(model.EventVerificationCompleted), events.count(model.EventArtifactCommitted))
	}
	var slide blueprint.Slide
	if err := json.Unmarshal(readFile(t, filepath.Join(dir, model.SlideJSONPath("s1"))), &slide); err != nil {
		t.Fatal(err)
	}
	if slide.Title != "增长战略" || slide.Revision != 2 {
		t.Fatalf("slide=%+v", slide)
	}
}

func TestDirectVerifyFailureDoesNotCommit(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "修改文本为：增长战略", model.MaterializationFresh)
	writePackArtifacts(t, dir, pack)
	formal := filepath.Join(dir, model.SlideHTMLPath("s1"))
	before := readFile(t, formal)
	commits := 0
	executor := executorFunc(func(_ context.Context, input StepInput) (StepResult, error) {
		ref := presentationSlideRef("s1")
		if _, err := input.Transaction.Stage(ref, input.Step.ID, []byte("<html>broken</html>")); err != nil {
			return StepResult{}, err
		}
		return StepResult{Summary: "staged invalid html", Artifacts: []ArtifactRef{ref}, Issues: []Issue{}}, nil
	})
	outcome := NewAdaptiveRuntime(executor).Run(context.Background(), RuntimeInput{
		RunID: "verify-fail", ProjectDir: dir, Context: pack,
		CommitMetadata: func(context.Context, ChangeSet) error { commits++; return nil },
	})
	if outcome.Status != StatusFailed || outcome.Code != CodeVerificationFailed {
		t.Fatalf("outcome=%+v", outcome)
	}
	if commits != 0 || string(before) != string(readFile(t, formal)) {
		t.Fatal("failed direct verification committed formal state")
	}
	if _, err := os.Stat(filepath.Join(dir, ".staging", "verify-fail")); !os.IsNotExist(err) {
		t.Fatalf("staging was not abandoned: %v", err)
	}
}

func TestDirectScopeExpansionAbandonsAndUpgradesToCompact(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(model.ArtifactBlueprint, model.TargetSlide, model.IntentApply, "修改标题为：新标题", model.MaterializationFresh)
	writePackArtifacts(t, dir, pack)
	events := &eventRecorder{}
	delegate := CognitiveExecutor{}
	executor := executorFunc(func(ctx context.Context, input StepInput) (StepResult, error) {
		if input.State.Strategy == StrategyDirectAction {
			return StepResult{Issues: []Issue{{Code: ErrDirectActionUpgrade.Error(), Severity: SeverityInfo}}}, ErrDirectActionUpgrade
		}
		return delegate.Execute(ctx, input)
	})
	outcome := NewAdaptiveRuntime(executor).Run(context.Background(), RuntimeInput{
		RunID: "upgrade-run", ProjectDir: dir, Context: pack, Emitter: events,
	})
	if outcome.Status != StatusCompleted || outcome.Strategy != StrategyCompactWorkflow {
		t.Fatalf("outcome=%+v", outcome)
	}
	if events.count(model.EventStrategySelected) != 2 || events.count(model.EventPlanCreated) != 1 {
		t.Fatalf("strategy events=%d plans=%d", events.count(model.EventStrategySelected), events.count(model.EventPlanCreated))
	}
	if _, err := os.Stat(filepath.Join(dir, ".staging", "upgrade-run")); !os.IsNotExist(err) {
		t.Fatalf("staging was not cleaned: %v", err)
	}
}

func TestToolDisclosureIntersectsIntentStrategyStageAndStep(t *testing.T) {
	pack := testPack(model.ArtifactBlueprint, model.TargetSlide, model.IntentApply, "修改标题", model.MaterializationFresh)
	registry := DefaultToolRegistry(pack)
	step := directStep(pack)
	state := WorkflowState{
		WorkSpec: pack.WorkSpec, Strategy: StrategyDirectAction, Stage: StageExecute, Plan: WorkflowPlan{Operation: OperationRevise},
	}
	selected := ToolSelector{Registry: registry}.Select(state, step, RiskPolicy{})
	names := descriptorNames(selected.Descriptors)
	if !names["patch_staged_slide_blueprint"] || names["write_staged_slide_blueprint"] || names["write_staged_deck_blueprint"] || names["write_staged_presentation"] {
		t.Fatalf("unexpected direct disclosure: %v", names)
	}
	state.Strategy = StrategyRespond
	selected = ToolSelector{Registry: registry}.Select(state, step, RiskPolicy{})
	for _, descriptor := range selected.Descriptors {
		if descriptor.Risk == RiskWrite || descriptor.Risk == RiskDestructive {
			t.Fatalf("respond disclosed write tool %s", descriptor.Name)
		}
	}
	state.Strategy = StrategyDirectAction
	state.Stage = StageVerify
	if got := (ToolSelector{Registry: registry}).Select(state, step, RiskPolicy{}); len(got.Descriptors) != 0 {
		t.Fatalf("execute tools leaked into verify stage: %+v", descriptorNames(got.Descriptors))
	}

	presentationPack := testPack(model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "修改文本为：新消息", model.MaterializationFresh)
	presentationStep := directStep(presentationPack)
	presentationState := WorkflowState{
		WorkSpec: presentationPack.WorkSpec, Strategy: StrategyDirectAction, Stage: StageExecute,
		Plan: WorkflowPlan{Operation: OperationRevise},
	}
	presentationNames := descriptorNames((ToolSelector{Registry: DefaultToolRegistry(presentationPack)}).
		Select(presentationState, presentationStep, RiskPolicy{}).Descriptors)
	if !presentationNames["patch_staged_presentation"] || presentationNames["write_staged_presentation"] ||
		presentationNames["search_assets"] || presentationNames["write_staged_design_spec"] {
		t.Fatalf("unexpected presentation direct disclosure: %v", presentationNames)
	}
}

func TestCompactUsesDeterministicPlanAndRepairIsBounded(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "重建当前页结构", model.MaterializationFresh)
	writePackArtifacts(t, dir, pack)
	planner := &countingPlanModel{}
	runtime := NewAdaptiveRuntime(CognitiveExecutor{})
	runtime.Workflow.Planner.Model = planner
	outcome := runtime.Run(context.Background(), RuntimeInput{
		RunID: "compact-run", ProjectDir: dir, Context: pack,
	})
	if outcome.Status != StatusCompleted || outcome.Strategy != StrategyCompactWorkflow {
		t.Fatalf("outcome=%+v", outcome)
	}
	if planner.calls != 0 {
		t.Fatalf("deterministic compact workflow invoked planner %d times", planner.calls)
	}
	if outcome.RepairRounds > 2 {
		t.Fatalf("repair rounds exceeded limit: %d", outcome.RepairRounds)
	}
}

func TestDirectPresentationPatchesUniqueAnchor(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "修改文本为：增长战略", model.MaterializationFresh)
	writePackArtifacts(t, dir, pack)
	runtime := NewAdaptiveRuntime(CognitiveExecutor{})
	outcome := runtime.Run(context.Background(), RuntimeInput{
		RunID: "direct-presentation", ProjectDir: dir, Context: pack,
	})
	if outcome.Status != StatusCompleted || outcome.Strategy != StrategyDirectAction {
		t.Fatalf("outcome=%+v", outcome)
	}
	html := string(readFile(t, filepath.Join(dir, model.SlideHTMLPath("s1"))))
	if !strings.Contains(html, "增长战略") || strings.Contains(html, "Old message") {
		t.Fatalf("presentation patch was not local: %s", html)
	}
}

func TestCompactRepairsInvalidStagedPresentationOnce(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "重建当前页结构", model.MaterializationFresh)
	writePackArtifacts(t, dir, pack)
	delegate := CognitiveExecutor{}
	executor := executorFunc(func(ctx context.Context, input StepInput) (StepResult, error) {
		if input.State.Stage == StageExecute &&
			(input.Step.Kind == StepMaterializeSlide || input.Step.Kind == StepReviseSlide) {
			ref := presentationSlideRef("s1")
			if _, err := input.Transaction.Stage(ref, input.Step.ID, []byte("<html>broken</html>")); err != nil {
				return StepResult{}, err
			}
			return StepResult{Summary: "invalid first attempt", Artifacts: []ArtifactRef{ref}, Issues: []Issue{}}, nil
		}
		return delegate.Execute(ctx, input)
	})
	events := &eventRecorder{}
	outcome := NewAdaptiveRuntime(executor).Run(context.Background(), RuntimeInput{
		RunID: "compact-repair", ProjectDir: dir, Context: pack, Emitter: events,
	})
	if outcome.Status != StatusCompleted || outcome.RepairRounds != 1 {
		t.Fatalf("outcome=%+v", outcome)
	}
	if events.count(model.EventRepairStarted) != 1 || events.count(model.EventRepairCompleted) != 1 {
		t.Fatalf("repair events started=%d completed=%d", events.count(model.EventRepairStarted), events.count(model.EventRepairCompleted))
	}
}

func TestFullPEVUsesPlannerAndStructuredPlan(t *testing.T) {
	dir := t.TempDir()
	pack := testPack(model.ArtifactPresentation, model.TargetDeck, model.IntentApply, "统一全局风格", model.MaterializationFresh)
	writePackArtifacts(t, dir, pack)
	planner := &countingPlanModel{}
	events := &eventRecorder{}
	runtime := NewAdaptiveRuntime(CognitiveExecutor{})
	runtime.Workflow.Planner.Model = planner
	outcome := runtime.Run(context.Background(), RuntimeInput{
		RunID: "full-pev", ProjectDir: dir, Context: pack, Emitter: events,
	})
	if outcome.Status != StatusCompleted || outcome.Strategy != StrategyFullPEV {
		t.Fatalf("outcome=%+v", outcome)
	}
	if planner.calls != 1 || events.count(model.EventPlanCreated) != 1 {
		t.Fatalf("planner calls=%d plan events=%d", planner.calls, events.count(model.EventPlanCreated))
	}
}

func TestFourTargetPlaybooksRemainAvailable(t *testing.T) {
	cases := []struct {
		artifact model.Artifact
		level    model.TargetLevel
		key      string
	}{
		{model.ArtifactBlueprint, model.TargetDeck, "blueprint/deck"},
		{model.ArtifactBlueprint, model.TargetSlide, "blueprint/slide"},
		{model.ArtifactPresentation, model.TargetDeck, "presentation/deck"},
		{model.ArtifactPresentation, model.TargetSlide, "presentation/slide"},
	}
	for _, tc := range cases {
		spec := testPack(tc.artifact, tc.level, model.IntentApply, "执行目标", model.MaterializationFresh).WorkSpec
		book, err := PlaybookFor(spec)
		if err != nil {
			t.Fatalf("%s/%s: %v", tc.artifact, tc.level, err)
		}
		if book.Key() != tc.key {
			t.Fatalf("playbook=%s want=%s", book.Key(), tc.key)
		}
	}
}

func TestUnknownStrategyDisclosesNoTools(t *testing.T) {
	pack := testPack(model.ArtifactPresentation, model.TargetSlide, model.IntentApply, "修改文本", model.MaterializationFresh)
	state := WorkflowState{
		WorkSpec: pack.WorkSpec, Strategy: ExecutionStrategy("unknown"), Stage: StageExecute,
		Plan: WorkflowPlan{Operation: OperationRevise},
	}
	if selected := (ToolSelector{Registry: DefaultToolRegistry(pack)}).
		Select(state, directStep(pack), RiskPolicy{}); len(selected.Descriptors) != 0 {
		t.Fatalf("unknown strategy disclosed tools: %v", descriptorNames(selected.Descriptors))
	}
}

func descriptorNames(descriptors []ToolDescriptor) map[string]bool {
	out := map[string]bool{}
	for _, descriptor := range descriptors {
		out[descriptor.Name] = true
	}
	return out
}

func testPack(artifact model.Artifact, level model.TargetLevel, intent model.InteractionIntent, instruction string, state model.MaterializationState) contextengine.ContextPack {
	slide := blueprint.Slide{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1, SlideID: "s1",
		SectionID: "section-1", Role: "evidence", Title: "Old title", KeyMessage: "Old message",
		Content:      blueprint.Content{Summary: "Old summary", Points: []string{"point"}},
		VisualIntent: blueprint.VisualIntent{Archetype: "content", Description: "simple slide", AssetQueries: []string{}},
		CreatedAt:    1, UpdatedAt: 1,
	}
	deck := blueprint.Deck{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1, ProjectID: "p1", Title: "Deck",
		Goal: "Goal", Audience: "Audience", Language: "zh-CN", CoreThesis: "Thesis", NarrativeArc: "Arc",
		Sections:   []blueprint.Section{{ID: "section-1", Number: "01", Title: "Main", Subsections: []blueprint.Subsection{}}},
		SlideOrder: []string{"s1"}, CreatedAt: 1, UpdatedAt: 1,
	}
	target := model.RunTarget{Artifact: artifact, Level: level}
	if level == model.TargetSlide {
		target.SlideID = "s1"
	}
	spec := model.WorkSpec{
		Target: target, Interaction: model.RunInteraction{Intent: intent, Clarification: model.ClarifyNever},
		Instruction: instruction,
	}
	materialization := &blueprint.Materialization{State: string(state), Revisions: model.MaterializationRevisions{
		Presentation: 1, Deck: 1, SlideBlueprint: 1, Design: 1,
	}}
	return contextengine.ContextPack{
		SchemaVersion: contextengine.SchemaVersion, WorkSpec: spec,
		Project: contextengine.ProjectContext{ID: "p1", Title: "Deck"},
		Deck: contextengine.DeckContext{Deck: deck, Summaries: []contextengine.SlideSummary{{
			ID: "s1", SectionID: "section-1", Role: "evidence", Title: slide.Title, KeyMessage: slide.KeyMessage,
		}}},
		Target: contextengine.TargetContext{
			Artifact: artifact, Level: level, Slide: &slide, Materialization: materialization,
		},
		Presentation: contextengine.PresentationContext{Summaries: map[string]contextengine.HTMLSummary{}},
		Assets:       []contextengine.AssetCandidate{}, RelatedSlides: []contextengine.SlideSummary{},
		RecentTurns: []contextengine.RecentTurn{},
		Revisions: contextengine.RevisionRefs{
			Deck: 1, Design: 1, Slides: map[string]int{"s1": 1}, Presentations: map[string]int{"s1": 1},
		},
		Manifest: contextengine.ContextManifest{
			ContextID: "ctx-test", RunID: "run", ThreadID: "thread", ProjectID: "p1",
			Profile:  contextengine.ProfileID(string(artifact) + "/" + string(level)),
			ReadOnly: true, BudgetTokens: 20000, EstimatedTokens: 1000,
			Segments: []contextengine.ContextSegment{}, Refs: []contextengine.ContextRef{},
			Dropped: []contextengine.DroppedSegment{}, Warnings: []string{},
		},
	}
}

func writePackArtifacts(t *testing.T, dir string, pack contextengine.ContextPack) {
	t.Helper()
	writeJSONFile(t, filepath.Join(dir, "deck.json"), pack.Deck.Deck)
	if pack.Target.Slide != nil {
		writeJSONFile(t, filepath.Join(dir, model.SlideJSONPath(pack.Target.Slide.SlideID)), pack.Target.Slide)
	}
	html := defaultSlideHTML(pack.Target.Slide.Title, pack.Target.Slide.KeyMessage)
	path := filepath.Join(dir, model.SlideHTMLPath("s1"))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
