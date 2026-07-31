package model

type RunContext struct {
	RunID           string
	ContextID       string
	Profile         string
	PackHash        string
	EstimatedTokens int
	BudgetTokens    int
	ManifestJSON    string
	CreatedAt       int64
}
