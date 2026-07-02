package model

// Project 是一个 PPT 项目（合并原 Deck）。M1 仅用到隔离/加锁所需字段。
type Project struct {
	ID         string
	Title      string
	WorkDir    string
	Theme      string
	Status     string
	DesignPath string
	CreatedAt  int64
	UpdatedAt  int64
}

// Thread 是一条对话线程；Run 挂在其下，提供 project 归属。
type Thread struct {
	ID          string
	ProjectID   string
	Title       string
	HistoryPath string
	Status      string
	CreatedAt   int64
	UpdatedAt   int64
}
