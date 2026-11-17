package service

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// SlideService provides the current slide's rendered content.
type SlideService struct {
	store          store.Store
	themes         *ThemeService
	renderer       workflow.SlideRenderer
	thumbnailSlots chan struct{}
}

func NewSlideService(s store.Store, themes *ThemeService, renderer workflow.SlideRenderer) *SlideService {
	return &SlideService{store: s, themes: themes, renderer: renderer, thumbnailSlots: make(chan struct{}, 1)}
}
