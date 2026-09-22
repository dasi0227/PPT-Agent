package service

import "github.com/dasi0227/PPT-Agent/backend/internal/store"

// SlideService provides the current slide's rendered content.
type SlideService struct {
	store  store.Store
	themes *ThemeService
}

func NewSlideServiceWithThemes(s store.Store, themes *ThemeService) *SlideService {
	service := NewSlideService(s)
	service.themes = themes
	return service
}

func NewSlideService(s store.Store) *SlideService {
	return &SlideService{store: s}
}
