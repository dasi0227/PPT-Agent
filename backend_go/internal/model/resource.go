package model

import "fmt"

// Resource is the database-owned identity and metadata of a file-backed asset.
type Resource struct {
	Type           string   `json:"type"`
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	NormalizedName string   `json:"-"`
	Description    string   `json:"description"`
	Disabled       bool     `json:"disabled"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
	Tags           []string `json:"tags"`
}

type ResourceContentState struct {
	ContentState string `json:"content_state"`
	ContentError string `json:"content_error,omitempty"`
}

// Repository loading treats a missing body separately from an unusable body.
// Catalog inspection may still expose either state for human management.
func RepositoryContentError(kind, id string, state ResourceContentState) *AgentError {
	if state.ContentState == "ready" {
		return nil
	}
	code := CodeInternal
	if state.ContentState == "missing" {
		code = CodeNotFound
	}
	reason := fmt.Sprintf("%s %q body has unusable content state %q", kind, id, state.ContentState)
	if state.ContentState == "missing" {
		reason = fmt.Sprintf("%s %q body file does not exist", kind, id)
	}
	if state.ContentError != "" {
		reason += ": " + state.ContentError
	}
	err := NewAgentError(code, "load_"+kind, fmt.Errorf("%s", reason))
	err.Reason = reason
	if code != CodeInternal {
		err.Measure = "Restore the missing " + kind + " body file or choose an existing " + kind + " ID from the repository catalog. Do not repeat an unchanged load."
	}
	return err
}

func RepositoryDisabledError(kind, id string) *AgentError {
	reason := fmt.Sprintf("%s %q is disabled and is not disclosed for loading", kind, id)
	err := NewAgentError(CodeResourceNotDisclosed, "load_"+kind, fmt.Errorf("%s", reason))
	err.Reason = reason
	err.Measure = "Choose an enabled " + kind + " ID from the disclosed repository catalog. Do not load disabled entries or retry unchanged IDs."
	return err
}

type Snippet struct {
	Resource
	ResourceContentState
	Content string `json:"content"`
	OpenURL string `json:"open_url"`
}
