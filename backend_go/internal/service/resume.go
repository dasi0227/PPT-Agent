package service

import (
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func (svc *RunService) resumeProvider(runModel model.Run, saved ...*llm.RouteState) (llm.Provider, error) {
	if svc.registry == nil {
		return nil, errors.New("model registry is required to resume run")
	}
	snapshot := svc.registry.Snapshot()
	if pinned, ok := svc.snapshots.Load(runModel.ID); ok {
		snapshot = pinned.(*llm.Registry)
	}
	profile, err := snapshot.RoutedProfile("main", runModel.Model.ProfileName)
	if err != nil {
		return nil, err
	}
	if profile.ProviderName() != runModel.Model.Provider || profile.Model() != runModel.Model.Model || profile.URL() != runModel.Model.URL {
		return nil, errors.New("model profile changed since checkpoint")
	}
	route := profile.Adapter().(*llm.RoutedProvider)
	if len(saved) > 0 && saved[0] != nil {
		if err := route.Restore(*saved[0]); err != nil {
			return nil, err
		}
	} else if _, pinned := svc.snapshots.Load(runModel.ID); !pinned && snapshot.Public().Revision != "" {
		return nil, errors.New("原任务没有可恢复的模型快照，请新建一轮。")
	}
	return route, nil
}
