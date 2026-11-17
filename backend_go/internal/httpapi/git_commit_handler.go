package httpapi

import "github.com/dasi0227/PPT-Agent/backend/internal/service"

type GitCommitHandler struct{ svc *service.GitCommitService }

func NewGitCommitHandler(svc *service.GitCommitService) *GitCommitHandler {
	return &GitCommitHandler{svc: svc}
}
