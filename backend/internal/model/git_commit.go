package model

type GitCommitResult struct {
	ModelProfile string   `json:"model_profile"`
	FallbackUsed bool     `json:"fallback_used"`
	Title        string   `json:"title"`
	Items        []string `json:"items"`
	Branch       string   `json:"branch"`
	Hash         string   `json:"hash"`
	FilesChanged int      `json:"files_changed"`
	Insertions   int      `json:"insertions"`
	Deletions    int      `json:"deletions"`
	CommittedAt  string   `json:"committed_at"`
}
