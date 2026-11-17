package model

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ReviewResult preserves the reviewer's complete assessment, independently of
// tool execution status. The reviewer generates text; the backend wraps it.
type ReviewResult struct {
	Result string `json:"result"`
}

func (r ReviewResult) Validate() error {
	if !utf8.ValidString(r.Result) {
		return errors.New("review result must be valid UTF-8 text")
	}
	if strings.TrimSpace(r.Result) == "" {
		return errors.New("review result cannot be blank")
	}
	return nil
}
