package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

var (
	ErrPromptNotFound     = errors.New("service: prompt not found")
	ErrPromptNameConflict = errors.New("service: prompt name conflict")
	ErrPromptInvalid      = errors.New("service: invalid prompt")
)

type PromptValidationError struct {
	Field   string
	Message string
}

func (e *PromptValidationError) Error() string { return e.Message }
func (e *PromptValidationError) Is(target error) bool {
	return target == ErrPromptInvalid
}

type PromptNameConflictError struct {
	Field string
}

func (e *PromptNameConflictError) Error() string {
	return "prompt " + e.Field + " already exists"
}
func (e *PromptNameConflictError) Is(target error) bool {
	return target == ErrPromptNameConflict
}

type PromptWriteParams struct {
	Name  string
	Desc  string
	Value string
	Tags  []model.PromptTag
}

var defaultPromptSeeds = []PromptWriteParams{
	{
		Name: "叙事大纲 / Storyline Outline", Desc: "梳理整份演示的叙事主线与逐页表达任务。",
		Value: "请先梳理整份演示的叙事主线，明确开场、论证、转折与结论，并给出逐页标题和每页唯一表达任务。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		Name: "单页撰写 / Slide Draft", Desc: "围绕当前页面结论撰写简洁、聚焦的内容。",
		Value: "请围绕当前页面的核心结论撰写内容，保持标题结论化、正文简洁，并让所有信息共同支撑同一个表达任务。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		Name: "高管摘要 / Executive Summary", Desc: "提炼核心结论、关键数据、风险与下一步行动。",
		Value: "请将以上内容整理为高管摘要，优先呈现核心结论、关键数据、主要风险与下一步行动。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		Name: "精简改写 / Concise Rewrite", Desc: "删减重复和空泛表达，保留关键事实。",
		Value: "请在不损失关键事实的前提下精简这段内容，删除重复信息和空泛修饰，使表达更直接。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		Name: "深度分析 / Deep Analysis", Desc: "分析关键矛盾、因果关系、假设与风险。",
		Value: "请分析当前材料中的关键矛盾、因果关系、隐含假设与潜在风险，并给出有证据支撑的结论。",
		Tags:  []model.PromptTag{model.PromptTagReview},
	},
	{
		Name: "数据洞察 / Data Insights", Desc: "识别数据中的趋势、差异、异常与驱动因素。",
		Value: "请从数据中识别最重要的趋势、差异、异常与驱动因素，提炼适合在演示文稿中突出表达的洞察。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		Name: "图表建议 / Chart Recommendation", Desc: "根据数据关系选择最合适的图表表达。",
		Value: "请根据数据关系选择合适的图表类型，突出最重要的差异或趋势，并避免无意义的装饰和重复标签。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
	{
		Name: "视觉审查 / Visual Review", Desc: "检查并修复页面的视觉层级与排版问题。",
		Value: "请审查当前页面的视觉层级、网格对齐、留白比例、文字换行和内容密度，并直接修复影响阅读的问题。",
		Tags:  []model.PromptTag{model.PromptTagReview},
	},
	{
		Name: "内容审查 / Content Review", Desc: "检查事实、逻辑、重复信息与措辞问题。",
		Value: "请检查当前内容是否存在事实冲突、逻辑跳跃、信息重复、结论缺失或措辞含糊，并逐项修正。",
		Tags:  []model.PromptTag{model.PromptTagReview},
	},
	{
		Name: "演讲备注 / Speaker Notes", Desc: "补充重点解释、停顿和页面过渡话术。",
		Value: "请为当前页面补充简洁的演讲备注，包括开场衔接、重点解释、建议停顿和下一页过渡语。",
		Tags:  []model.PromptTag{model.PromptTagDeliverable},
	},
}

type promptStore interface {
	CreatePrompt(ctx context.Context, prompt model.Prompt, normalizedName string) error
	GetPrompt(ctx context.Context, id string) (model.Prompt, error)
	ListPrompts(ctx context.Context) ([]model.Prompt, error)
	UpdatePrompt(ctx context.Context, prompt model.Prompt, normalizedName string) error
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
	clean, normalizedName, err := validatePrompt(params)
	if err != nil {
		return model.Prompt{}, err
	}
	now := svc.clock()
	prompt := model.Prompt{
		ID: svc.newID(), Name: clean.Name, Desc: clean.Desc, Value: clean.Value,
		Tags: clean.Tags, CreatedAt: now, UpdatedAt: now,
	}
	if err := svc.store.CreatePrompt(ctx, prompt, normalizedName); err != nil {
		return model.Prompt{}, mapPromptStoreError(err)
	}
	return prompt, nil
}

func (svc *PromptService) SeedDefaults(ctx context.Context) (int, error) {
	created := 0
	for _, seed := range defaultPromptSeeds {
		if _, err := svc.Create(ctx, seed); err != nil {
			if errors.Is(err, ErrPromptNameConflict) {
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
	clean, normalizedName, err := validatePrompt(params)
	if err != nil {
		return model.Prompt{}, err
	}
	existing, err := svc.Get(ctx, id)
	if err != nil {
		return model.Prompt{}, err
	}
	prompt := model.Prompt{
		ID: id, Name: clean.Name, Desc: clean.Desc, Value: clean.Value,
		Tags: clean.Tags, Disabled: existing.Disabled, CreatedAt: existing.CreatedAt, UpdatedAt: svc.clock(),
	}
	if err := svc.store.UpdatePrompt(ctx, prompt, normalizedName); err != nil {
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
	params.Name = strings.TrimSpace(params.Name)
	params.Desc = strings.TrimSpace(params.Desc)
	params.Value = strings.TrimSpace(params.Value)
	switch {
	case params.Name == "":
		return params, "", invalidPrompt("name", "提示词名称不能为空")
	case utf8.RuneCountInString(params.Name) > 80:
		return params, "", invalidPrompt("name", "提示词名称最多 80 个字符")
	case strings.ContainsAny(params.Name, "\r\n\t"):
		return params, "", invalidPrompt("name", "提示词名称不能包含换行或制表符")
	case params.Desc == "":
		return params, "", invalidPrompt("desc", "提示词描述不能为空")
	case utf8.RuneCountInString(params.Desc) > 500:
		return params, "", invalidPrompt("desc", "提示词描述最多 500 个字符")
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
	return params, strings.ToLower(params.Name), nil
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
	var conflict *store.PromptNameConflictError
	if errors.As(err, &conflict) {
		return &PromptNameConflictError{Field: conflict.Field}
	}
	return err
}
