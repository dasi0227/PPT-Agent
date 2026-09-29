package model

import (
	"errors"
	"strings"
)

// ReviewResult is an artifact assessment, independent of tool execution status.
type ReviewResult struct {
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
}

func (r ReviewResult) Validate() error {
	if r.Decision != "approve" && r.Decision != "revise" && r.Decision != "refuse" {
		return errors.New("review decision must be approve, revise or refuse")
	}
	if len(r.Reasons) == 0 {
		return errors.New("review reasons are required, including for approval")
	}
	for _, reason := range r.Reasons {
		if strings.TrimSpace(reason) == "" {
			return errors.New("review reasons cannot be blank")
		}
	}
	return nil
}
