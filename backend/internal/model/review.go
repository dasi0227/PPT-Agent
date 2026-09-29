package model

import (
	"errors"
	"strings"
)

// ReviewResult is an artifact assessment, independent of tool execution status.
type ReviewResult struct {
	Type    string   `json:"type"`
	Reasons []string `json:"reasons"`
}

func (r ReviewResult) Validate() error {
	if r.Type != "approve" && r.Type != "check" && r.Type != "refuse" {
		return errors.New("review type must be approve, check or refuse")
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
