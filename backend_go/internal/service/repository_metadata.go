package service

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func validateRepositoryMetadata(name, description string) (string, string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	switch {
	case name == "":
		return "", "", fmt.Errorf("%w: resource name is required", ErrRepositoryCorrupt)
	case utf8.RuneCountInString(name) > 80:
		return "", "", fmt.Errorf("%w: resource name exceeds 80 characters", ErrRepositoryCorrupt)
	case description == "":
		return "", "", fmt.Errorf("%w: resource description is required", ErrRepositoryCorrupt)
	case utf8.RuneCountInString(description) > 500:
		return "", "", fmt.Errorf("%w: resource description exceeds 500 characters", ErrRepositoryCorrupt)
	default:
		return name, description, nil
	}
}
