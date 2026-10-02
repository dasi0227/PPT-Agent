package workflow

import (
	"errors"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

var errResourceUnseen = errors.New("current resource has not been supplied to the model")

// Only versions actually delivered to the model may authorize an edit. Missing
// resources may be created; existing unseen resources must first be read.
func modelSeenResourceVersion(input DomainToolInput, resource Resource) (string, error) {
	key := "ppt/" + resource.Key()
	hash := input.SeenVersions[key]
	if current := visibleResourceHashes(input.Messages)[key]; hash == "" && current != "" {
		hash = current
	}
	if hash != "" {
		return hash, nil
	}
	ref, err := refForResource(input.Context, resource)
	if err != nil {
		return "", err
	}
	_, _, err = readArtifact(input.ProjectDir, input.Session, ref)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return "", errResourceUnseen
}

func (state *RunState) rememberResourceVersions(from int) {
	if state.seenVersions == nil {
		state.seenVersions = map[string]string{}
		from = 0
	}
	for key, hash := range visibleResourceHashes(state.messages[from:]) {
		state.seenVersions[key] = hash
	}
}

// Request preparation can remove restored image messages as well as append
// resource context. Compare metadata identities instead of message offsets so
// image deduplication cannot skip a new version or replay an older version.
func (state *RunState) rememberPreparedResourceVersions(previous []llm.Message) {
	if state.seenVersions == nil {
		state.rememberResourceVersions(0)
		return
	}
	seen := make(map[*llm.MessageMetadata]bool, len(previous))
	for _, message := range previous {
		seen[message.Metadata] = true
	}
	for _, message := range state.messages {
		if message.Metadata == nil || seen[message.Metadata] {
			continue
		}
		for key, hash := range visibleResourceHashes([]llm.Message{message}) {
			state.seenVersions[key] = hash
		}
	}
}
