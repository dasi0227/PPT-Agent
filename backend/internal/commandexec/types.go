package commandexec

import "time"

const PolicyVersion = "run-command-shell-v1"

type Outcome string

const (
	Allow   Outcome = "allow"
	Confirm Outcome = "confirm"
	Deny    Outcome = "deny"
)

// Classification controls sandbox capabilities; the shell alone interprets source.
type Decision struct {
	Outcome      Outcome
	Mutates      bool
	Network      bool
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
