package sqlite

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type tagPO struct {
	ID        string `gorm:"column:id"`
	Scope     string `gorm:"column:scope"`
	Key       string `gorm:"column:key"`
	SortOrder int    `gorm:"column:sort_order"`
}

func (tagPO) TableName() string { return "tags" }

type resourceTagPO struct {
	ResourceType string `gorm:"column:resource_type"`
	ResourceID   string `gorm:"column:resource_id"`
	TagID        string `gorm:"column:tag_id"`
}

func (resourceTagPO) TableName() string { return "resource_tags" }

type resourceStatePO struct {
	ResourceType string `gorm:"column:resource_type"`
	ResourceID   string `gorm:"column:resource_id"`
	Disabled     bool   `gorm:"column:disabled"`
	UpdatedAt    int64  `gorm:"column:updated_at"`
}

func (resourceStatePO) TableName() string { return "resource_states" }

func listResourceTagKeys(db *gorm.DB, resourceType, resourceID string) ([]string, error) {
	var keys []string
	err := db.Table("resource_tags").
		Select("tags.key").
		Joins("JOIN tags ON tags.id = resource_tags.tag_id").
		Where("resource_tags.resource_type = ? AND resource_tags.resource_id = ?", resourceType, resourceID).
		Order("tags.sort_order ASC, tags.key ASC").
		Pluck("tags.key", &keys).Error
	return keys, err
}

func replaceResourceTagKeys(db *gorm.DB, resourceType, resourceID string, tagKeys []string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Delete(&resourceTagPO{}).Error; err != nil {
			return err
		}
		if len(tagKeys) == 0 {
			return nil
		}
		var tags []tagPO
		if err := tx.Where("scope = ? AND key IN ?", resourceType, tagKeys).Find(&tags).Error; err != nil {
			return err
		}
		if len(tags) != len(tagKeys) {
			return store.ErrTagNotFound
		}
		rows := make([]resourceTagPO, 0, len(tags))
		for _, tag := range tags {
			rows = append(rows, resourceTagPO{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				TagID:        tag.ID,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (s *Store) ListResourceTagKeys(ctx context.Context, resourceType, resourceID string) ([]string, error) {
	return listResourceTagKeys(s.db.WithContext(ctx), resourceType, resourceID)
}

func (s *Store) ReplaceResourceTagKeys(ctx context.Context, resourceType, resourceID string, tagKeys []string) error {
	return replaceResourceTagKeys(s.db.WithContext(ctx), resourceType, resourceID, tagKeys)
}

func (s *Store) GetResourceDisabled(ctx context.Context, resourceType, resourceID string) (bool, error) {
	var state resourceStatePO
	result := s.db.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Limit(1).Find(&state)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	return state.Disabled, nil
}

func (s *Store) SetResourceDisabled(ctx context.Context, resourceType, resourceID string, disabled bool, updatedAt int64) error {
	state := resourceStatePO{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Disabled:     disabled,
		UpdatedAt:    updatedAt,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "resource_type"}, {Name: "resource_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"disabled":   disabled,
			"updated_at": updatedAt,
		}),
	}).Create(&state).Error
}

func (s *Store) DeleteResourceMetadata(ctx context.Context, resourceType, resourceID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Delete(&resourceTagPO{}).Error; err != nil {
			return err
		}
		return tx.Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Delete(&resourceStatePO{}).Error
	})
}
