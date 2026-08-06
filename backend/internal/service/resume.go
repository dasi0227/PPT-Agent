package service

import (
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func (svc *RunService) resumeProvider(runModel model.Run) (llm.Provider, error) {
	if svc.registry == nil {
		return nil, errors.New("model registry is required to resume run")
	}
	profile, err := svc.registry.Resolve(runModel.Model.ProfileName)
	if err != nil {
		return nil, err
	}
	if profile.ProviderName() != runModel.Model.Provider ||
		profile.Model() != runModel.Model.Model ||
		profile.URL() != runModel.Model.URL {
		return nil, errors.New("model profile changed since checkpoint")
	}
	provider := profile.Adapter()
	if provider == nil {
		return nil, errors.New("model provider is unavailable")
	}
	return provider, nil
}
