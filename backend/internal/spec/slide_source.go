package spec

import (
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ReadSlideSpec validates only this page and preserves its exact saved bytes.
// Outline membership is checked by the caller using its effective project view.
func ReadSlideSpec(read func(string) ([]byte, error), id string) ([]byte, error) {
	if !model.IsSlideID(id) {
		return nil, fmt.Errorf("%w: invalid slide ID %q", ErrInvalid, id)
	}
	raw, err := read(model.SlideSpecPath(id))
	if err != nil {
		return nil, err
	}
	if _, err := ParseStrictSourceJSON(raw, "spec"); err != nil {
		return nil, err
	}
	return raw, nil
}
