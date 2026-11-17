package sqlite

import (
	"context"
	"errors"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

type resourcePO struct {
	Type           string `gorm:"column:type;primaryKey"`
	ID             string `gorm:"column:id;primaryKey"`
	Name           string
	NormalizedName string
	Description    string
	Disabled       bool
	CreatedAt      int64
	UpdatedAt      int64
}

func resourceToPO(r model.Resource) resourcePO {
	return resourcePO{Type: r.Type, ID: r.ID, Name: r.Name, NormalizedName: r.NormalizedName, Description: r.Description, Disabled: r.Disabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (r resourcePO) model() model.Resource {
	return model.Resource{Type: r.Type, ID: r.ID, Name: r.Name, NormalizedName: r.NormalizedName, Description: r.Description, Disabled: r.Disabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (resourcePO) TableName() string { return "resources" }

func resourceError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return store.ErrResourceNotFound
	}
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return store.ErrResourceConflict
	}
	return err
}

func (s *Store) CreateResource(ctx context.Context, r model.Resource) error {
	return resourceError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := resourceToPO(r)
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return replaceResourceTagKeys(tx, r.Type, r.ID, r.Tags)
	}))
}

func (s *Store) GetResource(ctx context.Context, kind, id string) (model.Resource, error) {
	var row resourcePO
	if err := s.db.WithContext(ctx).Where("type = ? AND id = ?", kind, id).Take(&row).Error; err != nil {
		return model.Resource{}, resourceError(err)
	}
	r := row.model()
	tags, err := listResourceTagKeys(s.db.WithContext(ctx), kind, id)
	r.Tags = tags
	return r, err
}

func (s *Store) ListResources(ctx context.Context, kind string) ([]model.Resource, error) {
	var rows []resourcePO
	if err := s.db.WithContext(ctx).Where("type = ?", kind).Order("name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]model.Resource, 0, len(rows))
	for _, row := range rows {
		r := row.model()
		tags, err := listResourceTagKeys(s.db.WithContext(ctx), kind, r.ID)
		if err != nil {
			return nil, err
		}
		r.Tags = tags
		result = append(result, r)
	}
	return result, nil
}

func (s *Store) UpdateResource(ctx context.Context, r model.Resource) error {
	return resourceError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&resourcePO{}).Where("type = ? AND id = ?", r.Type, r.ID).Updates(map[string]any{"name": r.Name, "normalized_name": r.NormalizedName, "description": r.Description, "disabled": r.Disabled, "updated_at": r.UpdatedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return store.ErrResourceNotFound
		}
		return replaceResourceTagKeys(tx, r.Type, r.ID, r.Tags)
	}))
}

func (s *Store) DeleteResource(ctx context.Context, kind, id string) error {
	result := s.db.WithContext(ctx).Where("type = ? AND id = ?", kind, id).Delete(&resourcePO{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return store.ErrResourceNotFound
	}
	return nil
}
