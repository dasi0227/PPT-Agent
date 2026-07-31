package model

// Run 是一次 Agent 执行的领域表示（生命周期外壳的元数据）。
type Run struct {
	ID        string
	ThreadID  string
	ProjectID string
	WorkSpec  WorkSpec
	Status    RunStatus
	CreatedAt int64
	UpdatedAt int64
}

// CreateRunParams 是发起一次 Run 的入参（来自 API 层，已解析）。
type CreateRunParams struct {
	ThreadID    string
	ProjectID   string
	PageIndex   *int
	Instruction string
	// Blueprint/presentation runner implementation options.
	Brief      string
	SlideCount int
	Language   string
	// Theme falls back to project.Theme when omitted.
	Theme string
	// Internal deck runner page count.
	PageCount int
	WorkSpec  WorkSpec
}
