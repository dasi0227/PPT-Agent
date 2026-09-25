package workflow

import (
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// Journals retain the physical collection preimage; public changes and evidence
// retain per-slide identity. Reformatting the collection is not a page edit.
func appendSpecChanges(out *ChangeSet, entry sessionArtifact) {
	before, _ := spec.ParseCollection(entry.BeforeContent)
	after, _ := spec.ParseCollection(entry.AfterContent)
	ids := map[string]bool{}
	for id := range before {
		ids[id] = true
	}
	for id := range after {
		ids[id] = true
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		old, oldErr := spec.SpecEntry(before, id)
		next, nextErr := spec.SpecEntry(after, id)
		if oldErr == nil && nextErr == nil && hashBytes(old) == hashBytes(next) {
			continue
		}
		change := ArtifactChange{Artifact: specSlideRef(id), BeforeHash: hashBytes(old), AfterHash: hashBytes(next), Source: entry.Source, Tentative: strings.HasPrefix(entry.Source, "tentative:")}
		change.Insertions, change.Deletions = lineDiffStat(old, next)
		switch {
		case nextErr != nil:
			out.Deleted = append(out.Deleted, change)
		case oldErr != nil:
			out.Created = append(out.Created, change)
		default:
			out.Updated = append(out.Updated, change)
		}
	}
}
