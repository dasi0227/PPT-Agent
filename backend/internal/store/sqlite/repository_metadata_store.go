package sqlite

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
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
