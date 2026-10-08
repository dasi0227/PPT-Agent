package commandexec

import "time"

const PolicyVersion = "run-command-v3"

type Outcome string

const (
	Allow   Outcome = "allow"
	Confirm Outcome = "confirm"
	Deny    Outcome = "deny"
)

type Command struct {
	Args []string
}

type Pipeline struct {
	Commands []Command
}

type Graph struct {
	Groups []Pipeline
}

type Decision struct {
	Outcome      Outcome
	Mutates      bool
	Graph        Graph
	CommandHash  string
	Display      string
	ReasonCode   string
	PublicReason string
	TargetPaths  []string
	PreimageHash string
}

type Result struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	Duration        time.Duration
	OutputTruncated bool
}
