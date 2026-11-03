package sessionstate

import "github.com/dasi0227/PPT-Agent/backend/internal/model"

// CommandEffects are prepared outside SQLite and applied with the successful
// command result. Cancellation commits neither the candidate nor its result.
type CommandEffects struct {
	Context    *ContextUpdate
	Compaction *model.ContextCompaction
	Briefing   *model.BriefingVersion
	Rename     *RenameEffect
}

type RenameEffect struct {
	Title            string
	OperationVersion int64
	Automatic        bool
	Enabled          *bool
}
