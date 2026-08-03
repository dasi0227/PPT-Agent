// Package llmtest provides deterministic Provider fakes for Runtime tests.
package llmtest

import (
	"context"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

type FakeProvider struct {
	mu sync.Mutex

	ProviderName string
	ModelName    string
	Caps         llm.Capabilities
	Script       []llm.GenerateResponse
	Exhausted    llm.GenerateResponse
	GenerateErr  error

	requests []llm.GenerateRequest
}

var _ llm.Provider = (*FakeProvider)(nil)

func (f *FakeProvider) Name() string {
	if f.ProviderName == "" {
		return "fake"
	}
	return f.ProviderName
}

func (f *FakeProvider) Model() string {
	if f.ModelName == "" {
		return "fake-model"
	}
	return f.ModelName
}

func (f *FakeProvider) Capabilities() llm.Capabilities { return f.Caps }

func (f *FakeProvider) Requests() []llm.GenerateRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]llm.GenerateRequest(nil), f.requests...)
}

func (f *FakeProvider) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	if err := ctx.Err(); err != nil {
		return llm.GenerateResponse{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if f.GenerateErr != nil {
		return llm.GenerateResponse{}, f.GenerateErr
	}
	if len(f.Script) == 0 {
		return f.Exhausted, nil
	}
	response := f.Script[0]
	f.Script = f.Script[1:]
	return response, nil
}
