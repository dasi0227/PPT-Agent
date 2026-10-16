package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

type SuggestResult struct {
	Requests       []string           `json:"requests"`
	ModelExecution llm.ModelExecution `json:"model_execution"`
	PromptVersion  string             `json:"prompt_version"`
}

type SuggestService struct {
	store    store.Store
	registry *llm.Registry
	hub      *ThreadEventHub
}

func NewSuggestService(s store.Store, registry *llm.Registry, hub *ThreadEventHub) *SuggestService {
	return &SuggestService{store: s, registry: registry, hub: hub}
}

func (s *SuggestService) SetAutomatic(ctx context.Context, threadID string, enabled bool) (model.Thread, error) {
	thread, err := s.store.UpdateThreadSuggestState(ctx, threadID, enabled)
	if err == nil && s.hub != nil {
		s.hub.PublishUpdated(thread)
	}
	return thread, err
}

func (s *SuggestService) Generate(ctx context.Context, execution model.CommandExecution) (SuggestResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := s.store.CheckSuggestCommand(ctx, execution); err != nil {
		return SuggestResult{}, err
	}
	project, err := s.store.GetProject(ctx, execution.ProjectID)
	if err != nil {
		return SuggestResult{}, err
	}
	source, err := s.store.LoadThreadRenameContext(ctx, execution.ThreadID)
	if err != nil {
		return SuggestResult{}, err
	}
	outline, err := (contextengine.OutlineLoader{}).Load(project.WorkDir)
	if err != nil {
		return SuggestResult{}, err
	}
	// Include conversation and current content identity, not tool traces or paths.
	titles := []string{}
	for _, page := range spec.FlattenOutline(outline) {
		titles = append(titles, page.Slide.Title)
	}
	reference, err := json.Marshal(map[string]any{"activity": source.Activity, "plan": source.Plan, "project": map[string]any{"title": project.Title, "slides": titles}})
	if err != nil {
		return SuggestResult{}, err
	}
	profile, err := s.registry.RoutedProfile("suggest", "")
	if err != nil {
		return SuggestResult{}, err
	}
	prompt := prompts.MustLoad("command.suggest")
	session := llm.NewSubmissionSession("suggest")
	session.Check = func(ctx context.Context) error { return s.store.CheckSuggestCommand(ctx, execution) }
	var requests []string
	_, err = session.Generate(ctx, profile.Adapter(), llm.GenerateRequest{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(prompt.Body)},
		{Role: llm.RoleUser, Content: llm.TextContent(string(reference))},
	}, Tools: []llm.ToolSchema{tooldefinition.Get("suggest_request")}}, func(response llm.GenerateResponse) error {
		var parseErr error
		requests, parseErr = parseSuggestedRequests(response)
		return parseErr
	}, "Submit exactly one suggest_request call with a requests array of zero to three distinct single-line sentences, each at most 80 characters. Do not include ordinary text. Use an empty array if no useful suggestion is supported.")
	if err != nil {
		return SuggestResult{}, err
	}
	display := contextengine.ProjectPublicTextContext(project, string(reference))
	for index := range requests {
		requests[index] = model.PublicText(requests[index], display)
	}
	return SuggestResult{Requests: requests, ModelExecution: llm.ExecutionOf(profile.Adapter()), PromptVersion: prompt.Version}, nil
}

func parseSuggestedRequests(response llm.GenerateResponse) ([]string, error) {
	if err := llm.ValidateSubmissionEnvelope(response, "suggest_request", true); err != nil {
		return nil, err
	}
	args := response.ToolCalls[0].Args
	raw, ok := args["requests"].([]any)
	if typed, yes := args["requests"].([]string); yes {
		raw = make([]any, len(typed))
		for i := range typed {
			raw[i] = typed[i]
		}
		ok = true
	}
	invalid := func() ([]string, error) {
		return nil, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/requests", "Supply only requests: zero to three distinct non-empty single-line strings, each at most 80 characters.")
	}
	if len(args) != 1 || !ok || len(raw) > 3 {
		return invalid()
	}
	requests := make([]string, 0, len(raw))
	for _, item := range raw {
		rawText, ok := item.(string)
		text := strings.TrimSpace(rawText)
		if !ok || text == "" || utf8.RuneCountInString(text) > 80 {
			return invalid()
		}
		for _, char := range rawText {
			if unicode.IsControl(char) || unicode.In(char, unicode.Cf) {
				return invalid()
			}
		}
		for _, previous := range requests {
			if strings.EqualFold(previous, text) {
				return invalid()
			}
		}
		requests = append(requests, text)
	}
	return requests, nil
}
