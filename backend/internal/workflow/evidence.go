package workflow

import (
	"sort"
	"sync"
	"time"
)

type Evidence struct {
	ID              string                `json:"id"`
	Kind            string                `json:"kind"`
	Target          Resource              `json:"target"`
	SourceHash      string                `json:"source_hash"`
	ProducedAt      int64                 `json:"produced_at"`
	Fresh           bool                  `json:"fresh"`
	Data            map[string]any        `json:"data,omitempty"`
	Materialization *MaterializationProof `json:"-"`
}

// MaterializationProof is runtime-only evidence that a successful render used
// exactly these persisted inputs. It is never projected into public events.
type MaterializationProof struct {
	SlideID               string
	HTMLRevision          int
	SourceOutlineRevision int
	SourceSpecRevision    int
	SourceDesignRevision  int
	SourceHash            string
}

type EvidenceLedger struct {
	mu      sync.RWMutex
	entries []Evidence
	version int64
}

func NewEvidenceLedger() *EvidenceLedger {
	return &EvidenceLedger{entries: []Evidence{}}
}

func (l *EvidenceLedger) Record(entry Evidence) Evidence {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry.ProducedAt == 0 {
		entry.ProducedAt = time.Now().Unix()
	}
	entry.Fresh = true
	l.entries = append(l.entries, entry)
	l.version++
	return entry
}

func (l *EvidenceLedger) Invalidate(target Resource) {
	l.mu.Lock()
	defer l.mu.Unlock()
	changed := false
	for index := range l.entries {
		if l.entries[index].Target.Key() == target.Key() && l.entries[index].Fresh {
			l.entries[index].Fresh = false
			changed = true
		}
	}
	if changed {
		l.version++
	}
}

func (l *EvidenceLedger) Entries(_ ChangeSet) []Evidence {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Evidence, len(l.entries))
	for index, entry := range l.entries {
		out[index] = entry
	}
	return out
}

func (l *EvidenceLedger) HasFresh(target Resource, sourceHash, kind string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for index := len(l.entries) - 1; index >= 0; index-- {
		entry := l.entries[index]
		if entry.Fresh && entry.Target.Key() == target.Key() && entry.SourceHash == sourceHash && entry.Kind == kind {
			return true
		}
	}
	return false
}

func (l *EvidenceLedger) FreshMaterializationProof(target Resource, sourceHash string) (MaterializationProof, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for index := len(l.entries) - 1; index >= 0; index-- {
		entry := l.entries[index]
		if !entry.Fresh || entry.Kind != "render" || entry.Target.Key() != target.Key() ||
			entry.SourceHash != sourceHash || entry.Materialization == nil {
			continue
		}
		proof := *entry.Materialization
		if proof.SlideID == target.SlideID && proof.SourceHash == sourceHash {
			return proof, true
		}
	}
	return MaterializationProof{}, false
}

func (l *EvidenceLedger) Version() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.version
}

func sortedEvidenceKinds(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}
