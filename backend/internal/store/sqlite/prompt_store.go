package sqlite

import (
	"context"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

type promptPO struct {
	ID             string `gorm:"column:id"`
	Name           string `gorm:"column:name"`
	NormalizedName string `gorm:"column:normalized_name"`
	Desc           string `gorm:"column:desc"`
	Value          string `gorm:"column:value"`
	CreatedAt      int64  `gorm:"column:created_at"`
	UpdatedAt      int64  `gorm:"column:updated_at"`
}

func (promptPO) TableName() string { return "prompts" }

func promptToPO(prompt model.Prompt, normalizedName string) promptPO {
	return promptPO{
		ID: prompt.ID, Name: prompt.Name, NormalizedName: normalizedName,
		Desc: prompt.Desc, Value: prompt.Value,
		CreatedAt: prompt.CreatedAt, UpdatedAt: prompt.UpdatedAt,
	}
}

func promptTagKeys(tags []model.PromptTag) []string {
	keys := make([]string, len(tags))
	for index, tag := range tags {
		keys[index] = string(tag)
	}
	return keys
}

func (s *Store) promptToModel(ctx context.Context, po promptPO) (model.Prompt, error) {
	keys, err := s.ListResourceTagKeys(ctx, "prompt", po.ID)
	if err != nil {
		return model.Prompt{}, err
	}
	tags := make([]model.PromptTag, len(keys))
	for index, key := range keys {
		tags[index] = model.PromptTag(key)
	}
	disabled, err := s.GetResourceDisabled(ctx, "prompt", po.ID)
	if err != nil {
		return model.Prompt{}, err
	}
	return model.Prompt{
		ID: po.ID, Name: po.Name, Desc: po.Desc, Value: po.Value,
		Tags: tags, Disabled: disabled, CreatedAt: po.CreatedAt, UpdatedAt: po.UpdatedAt,
	}, nil
}

func promptConstraintError(err error) error {
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return err
	}
	if strings.Contains(err.Error(), "prompts.normalized_name") {
		return &store.PromptNameConflictError{Field: "name"}
	}
	return err
}

func (s *Store) CreatePrompt(ctx context.Context, prompt model.Prompt, normalizedName string) error {
	po := promptToPO(prompt, normalizedName)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&po).Error; err != nil {
			return err
		}
		if err := replaceResourceTagKeys(tx, "prompt", prompt.ID, promptTagKeys(prompt.Tags)); err != nil {
			return err
		}
		if prompt.Disabled {
			state := resourceStatePO{ResourceType: "prompt", ResourceID: prompt.ID, Disabled: true, UpdatedAt: prompt.UpdatedAt}
			return tx.Create(&state).Error
		}
		return nil
	})
	return promptConstraintError(err)
}

func (s *Store) GetPrompt(ctx context.Context, id string) (model.Prompt, error) {
	var po promptPO
	if err := s.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return model.Prompt{}, err
	}
	return s.promptToModel(ctx, po)
}

func (s *Store) ListPrompts(ctx context.Context) ([]model.Prompt, error) {
	var rows []promptPO
	if err := s.db.WithContext(ctx).Order("updated_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	prompts := make([]model.Prompt, 0, len(rows))
	for _, row := range rows {
		prompt, err := s.promptToModel(ctx, row)
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, prompt)
	}
	return prompts, nil
}

func (s *Store) UpdatePrompt(ctx context.Context, prompt model.Prompt, normalizedName string) error {
	po := promptToPO(prompt, normalizedName)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&promptPO{}).Where("id = ?", prompt.ID).Updates(map[string]any{
			"name": po.Name, "normalized_name": po.NormalizedName, "desc": po.Desc,
			"value": po.Value, "updated_at": po.UpdatedAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return replaceResourceTagKeys(tx, "prompt", prompt.ID, promptTagKeys(prompt.Tags))
	})
	return promptConstraintError(err)
}

func (s *Store) SetPromptDisabled(ctx context.Context, id string, disabled bool, updatedAt int64) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&promptPO{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	if err := s.SetResourceDisabled(ctx, "prompt", id, disabled, updatedAt); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&promptPO{}).Where("id = ?", id).Update("updated_at", updatedAt).Error
}

func (s *Store) DeletePrompt(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ?", id).Delete(&promptPO{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("resource_type = ? AND resource_id = ?", "prompt", id).Delete(&resourceTagPO{}).Error; err != nil {
			return err
		}
		return tx.Where("resource_type = ? AND resource_id = ?", "prompt", id).Delete(&resourceStatePO{}).Error
	})
}
