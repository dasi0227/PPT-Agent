package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type tagPO struct {
	Name           string
	NormalizedName string
	IsSystem       bool
	CreatedAt      int64
	UpdatedAt      int64
	ID             string `gorm:"column:id;primaryKey"`
	Scope          string `gorm:"column:scope"`
	Key            string `gorm:"column:key"`
	SortOrder      int    `gorm:"column:sort_order"`
}

func (tagPO) TableName() string { return "tags" }

type resourceTagPO struct {
	ResourceType string `gorm:"column:resource_type"`
	ResourceID   string `gorm:"column:resource_id"`
	TagID        string `gorm:"column:tag_id"`
}

func (resourceTagPO) TableName() string { return "resource_tags" }

func listResourceTagKeys(db *gorm.DB, resourceType, resourceID string) ([]string, error) {
	keys := []string{}
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

func (s *Store) ListTags(ctx context.Context, scope string) ([]model.TagDefinition, error) {
	query := s.db.WithContext(ctx).Order("scope ASC, sort_order ASC, key ASC")
	if scope != "" {
		query = query.Where("scope = ?", scope)
	}
	var rows []tagPO
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	tags := make([]model.TagDefinition, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, model.TagDefinition{ID: row.ID, Scope: row.Scope, Key: row.Key, Name: row.Name, NormalizedName: row.NormalizedName, IsSystem: row.IsSystem, SortOrder: row.SortOrder, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return tags, nil
}

// InitializeTags is called only by explicit resource initialization. Conflict
// handling preserves both user metadata edits and existing dictionary identity.
func (s *Store) InitializeTags(ctx context.Context, tags []model.TagDefinition) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, tag := range tags {
			switch tag.Scope {
			case "theme", "component", "skill", "snippet":
			default:
				return fmt.Errorf("invalid tag scope %q", tag.Scope)
			}
			name := strings.TrimSpace(tag.Name)
			if tag.ID == "" || tag.Key == "" || name == "" {
				return fmt.Errorf("tag requires id, key and name")
			}
			row := tagPO{ID: tag.ID, Scope: tag.Scope, Key: tag.Key, Name: name, NormalizedName: strings.ToLower(name), IsSystem: tag.IsSystem, SortOrder: tag.SortOrder, CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix()}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
