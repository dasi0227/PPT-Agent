package contextengine

import "unicode/utf8"

type TokenEstimator interface {
	Estimate(any) int
}

type StableTokenEstimator struct{}

func (StableTokenEstimator) Estimate(v any) int {
	b := stableJSON(v)
	runes := utf8.RuneCount(b)
	if runes == 0 {
		return 0
	}
	// Stable provider-independent estimate with a deliberate safety margin.
	return (runes+2)/3 + 8
}
