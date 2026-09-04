package contextengine

type TokenEstimator interface {
	Estimate(any) int
}

type StableTokenEstimator struct{}

func (StableTokenEstimator) Estimate(v any) int {
	if v == nil {
		return 0
	}
	return EstimateValueTokens(v) + 8
}
