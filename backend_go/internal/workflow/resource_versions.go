package workflow

import (
	"errors"
	"io/fs"
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

func (state *RunState) rememberResourceVersions() {
	// Recompute the whole chain: retained patches can depend on earlier reads,
	// and compacted reads must not remain authorized by checkpoint hashes.
	state.seenVersions = visibleResourceHashes(state.messages)
}
