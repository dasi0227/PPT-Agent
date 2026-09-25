package model

// TagDefinition is the database-owned resource tag dictionary.
type TagDefinition struct {
	ID             string `json:"id"`
	Scope          string `json:"scope"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	NormalizedName string `json:"-"`
	IsSystem       bool   `json:"is_system"`
	SortOrder      int    `json:"sort_order"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}
