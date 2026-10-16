package commandexec

import "time"

const PolicyVersion = "run-command-shell-v2"

type Outcome string

const (
	Allow   Outcome = "allow"
	Confirm Outcome = "confirm"
	Deny    Outcome = "deny"
)

// Decision binds the shell source and its proposed local changes to approval.
type Decision struct {
	Outcome      Outcome
	Mutates      bool
	Source       string
	CommandHash  string
	Display      string
	ReasonCode   string
	PublicReason string
	TargetPaths  []string
	PreimageHash string
	FullReadPath string
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

func (r Result) DurationMS() int64 { return r.Duration.Milliseconds() }
