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
	AutoSuggestEnabled     bool
	SuggestVersion         int64
	AutoRenameEnabled      bool
	NamingRevision         int64
	RenameOperationVersion int64
	RenameInputCount       int
	RenameFirstInputSeen   bool
	CreatedAt              int64
	UpdatedAt              int64
}

type ThreadNamingInput struct {
	ThreadID          string
	RunID             string
	InputID           string
	ExecutionRevision int64
}

// ThreadConversationContext projects original user inputs and final replies.
// Plan is separate from history and only used by commands that need it.
type ThreadConversationContext struct {
	Plan   *PublicPlan
	Rounds []ConversationRound
}

type ConversationRound struct {
	Messages []ConversationMessage
}

type ConversationMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}
