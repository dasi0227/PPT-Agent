package sessionstate

import (
	"regexp"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// RunAction identifies one user operation. Retries retain the original version
// and key, even if ordinary progress has since advanced the Run entity.
type RunAction struct {
	SceneRevision  int64  `json:"scene_revision,string"`
	EntityRevision int64  `json:"entity_revision,string"`
	RequestKey     string `json:"request_key"`
}

var runActionKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func (a RunAction) Valid() bool {
	return a.SceneRevision > 0 && a.EntityRevision > 0 && runActionKey.MatchString(a.RequestKey)
}

// Run is current state; Receipt is the immutable acknowledgement of the action.
// Neither lets a client skip the batches preceding the receipt cursor.
type RunMutation struct {
	Run     RunView   `json:"run"`
	Receipt Receipt   `json:"receipt"`
	Current model.Run `json:"-"`
}
