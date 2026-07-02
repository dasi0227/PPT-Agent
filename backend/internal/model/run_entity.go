package model

// Run 是一次 Agent 执行的领域表示（生命周期外壳的元数据）。
type Run struct {
	ID        string
	ThreadID  string // 可空：repo 类快操作无 thread
	ProjectID string // 冗余便于按项目加锁；repo scope 可空
	Kind      Kind
	Scope     Scope
	PageIndex *int // 针对页时的页序，可空
	Mode      Mode
	Command   string // 显式指令名，可空
	Status    RunStatus
	CreatedAt int64
	UpdatedAt int64
}

// CreateRunParams 是发起一次 Run 的入参（来自 API 层，已解析）。
type CreateRunParams struct {
	ThreadID    string
	ProjectID   string
	Kind        Kind
	Scope       Scope
	PageIndex   *int
	Mode        Mode
	Command     string
	Instruction string
}
