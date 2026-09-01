package model

type PromptTag string

const (
	PromptTagStructure PromptTag = "structure"
	PromptTagDraft     PromptTag = "draft"
	PromptTagRewrite   PromptTag = "rewrite"
	PromptTagSummarize PromptTag = "summarize"
	PromptTagAnalysis  PromptTag = "analysis"
	PromptTagData      PromptTag = "data"
	PromptTagVisual    PromptTag = "visual"
	PromptTagReview    PromptTag = "review"
	PromptTagOther     PromptTag = "other"
)

type Prompt struct {
	ID        string      `json:"id"`
	KeyZH     string      `json:"key_zh"`
	KeyEN     string      `json:"key_en"`
	Value     string      `json:"value"`
	Tags      []PromptTag `json:"tags"`
	CreatedAt int64       `json:"created_at"`
	UpdatedAt int64       `json:"updated_at"`
}
