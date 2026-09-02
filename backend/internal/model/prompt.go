package model

type PromptTag string

const (
	PromptTagIdentity    PromptTag = "identity"
	PromptTagDeliverable PromptTag = "deliverable"
	PromptTagConstraint  PromptTag = "constraint"
	PromptTagGit         PromptTag = "git"
	PromptTagReview      PromptTag = "review"
	PromptTagOther       PromptTag = "other"
)

type Prompt struct {
	ID        string      `json:"id"`
	KeyZH     string      `json:"key_zh"`
	KeyEN     string      `json:"key_en"`
	Value     string      `json:"value"`
	Tags      []PromptTag `json:"tags"`
	Disabled  bool        `json:"disabled"`
	CreatedAt int64       `json:"created_at"`
	UpdatedAt int64       `json:"updated_at"`
}
