package llm

// NormalizeHistory prepares current-protocol messages for replay and compaction.
// It omits Run-owned pixels, without translating historical tool protocols.
func NormalizeHistory(messages []Message) []Message {
	return WithoutRenderImages(WithoutRunImageMessages(messages))
}
