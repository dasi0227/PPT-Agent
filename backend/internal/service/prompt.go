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

var defaultPromptSeeds = []PromptWriteParams{
	{
		KeyZH: "叙事大纲", KeyEN: "storyline-outline",
		Value: "请先梳理整份演示的叙事主线，明确开场、论证、转折与结论，并给出逐页标题和每页唯一表达任务。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		KeyZH: "单页撰写", KeyEN: "slide-draft",
		Value: "请围绕当前页面的核心结论撰写内容，保持标题结论化、正文简洁，并让所有信息共同支撑同一个表达任务。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		KeyZH: "高管摘要", KeyEN: "executive-summary",
		Value: "请将以上内容整理为高管摘要，优先呈现核心结论、关键数据、主要风险与下一步行动。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		KeyZH: "精简改写", KeyEN: "concise-rewrite",
		Value: "请在不损失关键事实的前提下精简这段内容，删除重复信息和空泛修饰，使表达更直接。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		KeyZH: "深度分析", KeyEN: "deep-analysis",
		Value: "请分析当前材料中的关键矛盾、因果关系、隐含假设与潜在风险，并给出有证据支撑的结论。",
		Tags:  []model.PromptTag{model.PromptTagReview},
	},
	{
		KeyZH: "数据洞察", KeyEN: "data-insights",
		Value: "请从数据中识别最重要的趋势、差异、异常与驱动因素，提炼适合在演示文稿中突出表达的洞察。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		KeyZH: "图表建议", KeyEN: "chart-recommendation",
		Value: "请根据数据关系选择合适的图表类型，突出最重要的差异或趋势，并避免无意义的装饰和重复标签。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		KeyZH: "视觉审查", KeyEN: "visual-review",
		Value: "请审查当前页面的视觉层级、网格对齐、留白比例、文字换行和内容密度，并直接修复影响阅读的问题。",
		Tags:  []model.PromptTag{model.PromptTagReview},
	},
	{
		KeyZH: "内容审查", KeyEN: "content-review",
		Value: "请检查当前内容是否存在事实冲突、逻辑跳跃、信息重复、结论缺失或措辞含糊，并逐项修正。",
		Tags:  []model.PromptTag{model.PromptTagReview},
	},
	{
		KeyZH: "演讲备注", KeyEN: "speaker-notes",
		Value: "请为当前页面补充简洁的演讲备注，包括开场衔接、重点解释、建议停顿和下一页过渡语。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
}

type promptStore interface {
	CreatePrompt(ctx context.Context, prompt model.Prompt, normalizedKeyEN string) error
	GetPrompt(ctx context.Context, id string) (model.Prompt, error)
	ListPrompts(ctx context.Context) ([]model.Prompt, error)
	UpdatePrompt(ctx context.Context, prompt model.Prompt, normalizedKeyEN string) error
	SetPromptDisabled(ctx context.Context, id string, disabled bool, updatedAt int64) error
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

func (svc *PromptService) SeedDefaults(ctx context.Context) (int, error) {
	created := 0
	for _, seed := range defaultPromptSeeds {
		if _, err := svc.Create(ctx, seed); err != nil {
			if errors.Is(err, ErrPromptKeyConflict) {
				continue
			}
			return created, err
		}
		created++
	}
	return created, nil
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
		Tags: clean.Tags, Disabled: existing.Disabled, CreatedAt: existing.CreatedAt, UpdatedAt: svc.clock(),
	}
	if err := svc.store.UpdatePrompt(ctx, prompt, normalizedKeyEN); err != nil {
		return model.Prompt{}, mapPromptStoreError(err)
	}
	return prompt, nil
}

func (svc *PromptService) SetDisabled(ctx context.Context, id string, disabled bool) (model.Prompt, error) {
	if _, err := svc.Get(ctx, id); err != nil {
		return model.Prompt{}, err
	}
	if err := svc.store.SetPromptDisabled(ctx, id, disabled, svc.clock()); err != nil {
		return model.Prompt{}, mapPromptStoreError(err)
	}
	return svc.Get(ctx, id)
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
		model.PromptTagIdentity: true, model.PromptTagDeliverable: true, model.PromptTagConstraint: true,
		model.PromptTagGit: true, model.PromptTagReview: true, model.PromptTagOther: true,
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
