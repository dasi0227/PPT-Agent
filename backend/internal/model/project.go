package model

// Project 是一个 PPT 项目（合并原 Deck）。内容真相在文件系统，DB 仅存编排与修订游标；
// design.json / outline.json 路径由协议固定推导，不再入库。
type Project struct {
	ID              string
	Title           string
	WorkDir         string
	Theme           string
	Status          string
	OutlineRevision int
	DesignRevision  int
	LayoutVersion   int
	CreatedAt       int64
	UpdatedAt       int64
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
