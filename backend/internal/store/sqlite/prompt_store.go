package sqlite

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

type promptPO struct {
	ID              string `gorm:"column:id"`
	KeyZH           string `gorm:"column:key_zh"`
	KeyEN           string `gorm:"column:key_en"`
	NormalizedKeyEN string `gorm:"column:normalized_key_en"`
	Value           string `gorm:"column:value"`
	TagsJSON        string `gorm:"column:tags_json"`
	CreatedAt       int64  `gorm:"column:created_at"`
	UpdatedAt       int64  `gorm:"column:updated_at"`
}

func (promptPO) TableName() string { return "prompts" }

func promptToPO(prompt model.Prompt, normalizedKeyEN string) (promptPO, error) {
	tags, err := json.Marshal(prompt.Tags)
	if err != nil {
		return promptPO{}, err
	}
	return promptPO{
		ID: prompt.ID, KeyZH: prompt.KeyZH, KeyEN: prompt.KeyEN,
		NormalizedKeyEN: normalizedKeyEN, Value: prompt.Value,
		TagsJSON: string(tags), CreatedAt: prompt.CreatedAt, UpdatedAt: prompt.UpdatedAt,
	}, nil
}

func (po promptPO) toModel() (model.Prompt, error) {
	tags := make([]model.PromptTag, 0, 2)
	if err := json.Unmarshal([]byte(po.TagsJSON), &tags); err != nil {
		return model.Prompt{}, err
	}
	return model.Prompt{
		ID: po.ID, KeyZH: po.KeyZH, KeyEN: po.KeyEN, Value: po.Value,
		Tags: tags, CreatedAt: po.CreatedAt, UpdatedAt: po.UpdatedAt,
	}, nil
}

func promptConstraintError(err error) error {
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return err
	}
	field := "key"
	switch {
	case strings.Contains(err.Error(), "prompts.key_zh"):
		field = "key_zh"
	case strings.Contains(err.Error(), "prompts.normalized_key_en"):
		field = "key_en"
	}
	return &store.PromptKeyConflictError{Field: field}
}

func (s *Store) CreatePrompt(ctx context.Context, prompt model.Prompt, normalizedKeyEN string) error {
	po, err := promptToPO(prompt, normalizedKeyEN)
	if err != nil {
		return err
	}
	return promptConstraintError(s.db.WithContext(ctx).Create(&po).Error)
}

func (s *Store) GetPrompt(ctx context.Context, id string) (model.Prompt, error) {
	var po promptPO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Prompt{}, err
	}
	return po.toModel()
}

func (s *Store) ListPrompts(ctx context.Context) ([]model.Prompt, error) {
	var rows []promptPO
	if err := s.db.WithContext(ctx).Order("updated_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	prompts := make([]model.Prompt, 0, len(rows))
	for _, row := range rows {
		prompt, err := row.toModel()
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, prompt)
	}
	return prompts, nil
}

func (s *Store) UpdatePrompt(ctx context.Context, prompt model.Prompt, normalizedKeyEN string) error {
	po, err := promptToPO(prompt, normalizedKeyEN)
	if err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Model(&promptPO{}).Where("id = ?", prompt.ID).Updates(map[string]any{
		"key_zh": po.KeyZH, "key_en": po.KeyEN, "normalized_key_en": po.NormalizedKeyEN,
		"value": po.Value, "tags_json": po.TagsJSON, "updated_at": po.UpdatedAt,
	})
	if result.Error != nil {
		return promptConstraintError(result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) DeletePrompt(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&promptPO{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
