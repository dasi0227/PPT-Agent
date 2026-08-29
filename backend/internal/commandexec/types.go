package commandexec

import "time"

const PolicyVersion = "run-command-v1"

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

type AuditRecord struct {
	RunID         string   `json:"run_id"`
	CallID        string   `json:"call_id"`
	CommandHash   string   `json:"command_hash"`
	Command       string   `json:"command"`
	PolicyVersion string   `json:"policy_version"`
	Outcome       Outcome  `json:"outcome"`
	ReasonCode    string   `json:"reason_code,omitempty"`
	TargetPaths   []string `json:"target_paths"`
	Approval      string   `json:"approval,omitempty"`
	DurationMS    int64    `json:"duration_ms,omitempty"`
	ExitCode      int      `json:"exit_code,omitempty"`
	TimedOut      bool     `json:"timed_out,omitempty"`
	Truncated     bool     `json:"truncated,omitempty"`
	BeforeHash    string   `json:"before_hash,omitempty"`
	AfterHash     string   `json:"after_hash,omitempty"`
}
