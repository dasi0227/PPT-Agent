package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// ReviewResult is an artifact assessment, independent of tool execution status.
type ReviewResult struct {
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
}

// ParseReviewResult accepts one complete JSON object with exactly the business
// fields. Token decoding also rejects duplicate keys and case-folded field names.
func ParseReviewResult(text string) (ReviewResult, error) {
	var result ReviewResult
	if !utf8.ValidString(text) {
		return result, errors.New("review output must be valid UTF-8 JSON")
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return result, errors.New("review output must be a JSON object")
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return ReviewResult{}, err
		}
		key, ok := token.(string)
		if !ok || seen[key] || (key != "decision" && key != "reasons") {
			return ReviewResult{}, errors.New("review output allows only one decision and one reasons field")
		}
		seen[key] = true
		if key == "decision" {
			err = decoder.Decode(&result.Decision)
		} else {
			err = decoder.Decode(&result.Reasons)
		}
		if err != nil {
			return ReviewResult{}, fmt.Errorf("invalid review %s: %w", key, err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return ReviewResult{}, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ReviewResult{}, errors.New("review output must contain only one JSON object without surrounding text")
	}
	for i := range result.Reasons {
		result.Reasons[i] = strings.TrimSpace(result.Reasons[i])
	}
	if err := result.Validate(); err != nil {
		return ReviewResult{}, err
	}
	return result, nil
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
