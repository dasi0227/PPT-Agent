package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

var (
	ErrPromptNotFound    = errors.New("service: prompt not found")
	ErrPromptKeyConflict = errors.New("service: prompt key conflict")
	ErrPromptInvalid     = errors.New("service: invalid prompt")

	promptKeyZHPattern = regexp.MustCompile(`^[\p{Han}A-Za-z0-9_-]+$`)
	promptKeyENPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	promptHasHan       = regexp.MustCompile(`\p{Han}`)
)

type PromptValidationError struct {
	Field   string
	Message string
}

func (e *PromptValidationError) Error() string { return e.Message }
func (e *PromptValidationError) Is(target error) bool {
	return target == ErrPromptInvalid
}

type PromptKeyConflictError struct {
	Field string
}

func (e *PromptKeyConflictError) Error() string {
	return "prompt " + e.Field + " already exists"
}
func (e *PromptKeyConflictError) Is(target error) bool {
	return target == ErrPromptKeyConflict
}

type PromptWriteParams struct {
	KeyZH string
	KeyEN string
	Value string
	Tags  []model.PromptTag
}

type promptStore interface {
	CreatePrompt(ctx context.Context, prompt model.Prompt, normalizedKeyEN string) error
	GetPrompt(ctx context.Context, id string) (model.Prompt, error)
	ListPrompts(ctx context.Context) ([]model.Prompt, error)
	UpdatePrompt(ctx context.Context, prompt model.Prompt, normalizedKeyEN string) error
	DeletePrompt(ctx context.Context, id string) error
}

type PromptService struct {
	store promptStore
	clock func() int64
	newID func() string
}

func NewPromptService(store promptStore) *PromptService {
	return &PromptService{
		store: store,
		clock: func() int64 { return time.Now().Unix() },
		newID: func() string { return model.MustShortID("prm") },
	}
}

func (svc *PromptService) Create(ctx context.Context, params PromptWriteParams) (model.Prompt, error) {
	clean, normalizedKeyEN, err := validatePrompt(params)
	if err != nil {
		return model.Prompt{}, err
	}
	now := svc.clock()
	prompt := model.Prompt{
		ID: svc.newID(), KeyZH: clean.KeyZH, KeyEN: clean.KeyEN, Value: clean.Value,
		Tags: clean.Tags, CreatedAt: now, UpdatedAt: now,
	}
	if err := svc.store.CreatePrompt(ctx, prompt, normalizedKeyEN); err != nil {
		return model.Prompt{}, mapPromptStoreError(err)
	}
	return prompt, nil
}

func (svc *PromptService) Get(ctx context.Context, id string) (model.Prompt, error) {
	prompt, err := svc.store.GetPrompt(ctx, id)
	if err != nil {
		return model.Prompt{}, mapPromptStoreError(err)
	}
	return prompt, nil
}

func (svc *PromptService) List(ctx context.Context) ([]model.Prompt, error) {
	return svc.store.ListPrompts(ctx)
}

func (svc *PromptService) Update(ctx context.Context, id string, params PromptWriteParams) (model.Prompt, error) {
	clean, normalizedKeyEN, err := validatePrompt(params)
	if err != nil {
		return model.Prompt{}, err
	}
	existing, err := svc.Get(ctx, id)
	if err != nil {
		return model.Prompt{}, err
	}
	prompt := model.Prompt{
		ID: id, KeyZH: clean.KeyZH, KeyEN: clean.KeyEN, Value: clean.Value,
		Tags: clean.Tags, CreatedAt: existing.CreatedAt, UpdatedAt: svc.clock(),
	}
	if err := svc.store.UpdatePrompt(ctx, prompt, normalizedKeyEN); err != nil {
		return model.Prompt{}, mapPromptStoreError(err)
	}
	return prompt, nil
}

func (svc *PromptService) Delete(ctx context.Context, id string) error {
	return mapPromptStoreError(svc.store.DeletePrompt(ctx, id))
}

func validatePrompt(params PromptWriteParams) (PromptWriteParams, string, error) {
	params.KeyZH = strings.TrimSpace(params.KeyZH)
	params.KeyEN = strings.TrimSpace(params.KeyEN)
	params.Value = strings.TrimSpace(params.Value)
	switch {
	case params.KeyZH == "":
		return params, "", invalidPrompt("key_zh", "中文 key 不能为空")
	case utf8.RuneCountInString(params.KeyZH) > 32:
		return params, "", invalidPrompt("key_zh", "中文 key 最多 32 个字符")
	case !promptKeyZHPattern.MatchString(params.KeyZH) || !promptHasHan.MatchString(params.KeyZH):
		return params, "", invalidPrompt("key_zh", "中文 key 仅允许中英文、数字、连字符和下划线，且至少包含一个中文字符")
	case params.KeyEN == "":
		return params, "", invalidPrompt("key_en", "英文 key 不能为空")
	case utf8.RuneCountInString(params.KeyEN) > 64:
		return params, "", invalidPrompt("key_en", "英文 key 最多 64 个字符")
	case !promptKeyENPattern.MatchString(params.KeyEN):
		return params, "", invalidPrompt("key_en", "英文 key 仅允许英文字母、数字、连字符和下划线")
	case params.Value == "":
		return params, "", invalidPrompt("value", "提示词内容不能为空")
	case len([]byte(params.Value)) > 16*1024:
		return params, "", invalidPrompt("value", "提示词内容最多 16KB")
	case len(params.Tags) > 2:
		return params, "", invalidPrompt("tags", "每条提示词最多选择 2 个标签")
	}

	validTags := map[model.PromptTag]bool{
		model.PromptTagStructure: true, model.PromptTagDraft: true, model.PromptTagRewrite: true,
		model.PromptTagSummarize: true, model.PromptTagAnalysis: true, model.PromptTagData: true,
		model.PromptTagVisual: true, model.PromptTagReview: true, model.PromptTagOther: true,
	}
	seen := make(map[model.PromptTag]bool, len(params.Tags))
	for _, tag := range params.Tags {
		if !validTags[tag] {
			return params, "", invalidPrompt("tags", fmt.Sprintf("未知提示词标签 %q", tag))
		}
		if seen[tag] {
			return params, "", invalidPrompt("tags", "提示词标签不能重复")
		}
		seen[tag] = true
	}
	if params.Tags == nil {
		params.Tags = []model.PromptTag{}
	}
	return params, strings.ToLower(params.KeyEN), nil
}

func invalidPrompt(field, message string) error {
	return &PromptValidationError{Field: field, Message: message}
}

func mapPromptStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPromptNotFound
	}
	var conflict *store.PromptKeyConflictError
	if errors.As(err, &conflict) {
		return &PromptKeyConflictError{Field: conflict.Field}
	}
	return err
}
