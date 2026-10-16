package model

import "github.com/dasi0227/PPT-Agent/backend/internal/decoration"

// Project stores workspace identity and orchestration metadata.
// Authoring content lives in project files.
type Project struct {
	ID          string
	Title       string
	WorkDir     string
	Theme       string
	Decorations decoration.Positions
	Logo        *decoration.LogoMetadata
	CreatedAt   int64
	UpdatedAt   int64
}

// Thread 是一条对话线程；Run 挂在其下，提供 project 归属。
type Thread struct {
	ID                     string
	ProjectID              string
	Title                  string
	AutoRenameEnabled      bool
	NamingRevision         int64
	RenameOperationVersion int64
	RenameInputCount       int
	RenameFirstInputSeen   bool
	CreatedAt              int64
	UpdatedAt              int64
}

type ThreadNamingInput struct {
	ThreadID string
	InputID  string
}

type ThreadRenameContextSource struct {
	Activity []ThreadNamingActivity
	Plan     *PublicPlan
}

// ThreadNamingActivity keeps the order of recent user intent and final replies.
// Tool output, command cards and historical compaction content are not naming inputs.
type ThreadNamingActivity struct {
	Role string `json:"role"`
	Text string `json:"text"`
}
