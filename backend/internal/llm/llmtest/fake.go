// Package llmtest 提供 llm.Client 的脚本化测试替身，供 harness/run 确定性测试使用。
// 不在生产路径引用；避免测试打真实 DeepSeek（flaky）。
package llmtest

import (
	"context"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

// FakeClient 按预置脚本逐步返回 CallTool 结果；Stream 返回预置分片。
type FakeClient struct {
	mu sync.Mutex

	// Script 是 CallTool 的返回序列；按调用次序消费。耗尽后返回 ExhaustedResponse。
	Script []llm.ToolCallResponse
	// ExhaustedResponse 在脚本耗尽后返回（默认返回不含 finish 的空文本，制造 MAX_TURNS）。
	ExhaustedResponse llm.ToolCallResponse
	// CallToolErr 若非 nil，则 CallTool 直接返回该错误（测最小失败退出）。
	CallToolErr error
	// StreamChunks 是 Stream 的预置分片文本。
	StreamChunks []string
	// StreamBlockForever 为 true 时，Stream 在发完分片后阻塞，直到 ctx 取消（测泄漏）。
	StreamBlockForever bool

	callToolCount int
	chatResponses []string
	chatCount     int
}

var _ llm.Client = (*FakeClient)(nil)

func (f *FakeClient) CallToolCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callToolCount
}

func (f *FakeClient) Chat(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.chatCount < len(f.chatResponses) {
		r := f.chatResponses[f.chatCount]
		f.chatCount++
		return llm.ChatResponse{Content: r}, nil
	}
	return llm.ChatResponse{Content: ""}, nil
}

func (f *FakeClient) CallTool(ctx context.Context, _ llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	if err := ctx.Err(); err != nil {
		return llm.ToolCallResponse{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CallToolErr != nil {
		f.callToolCount++
		return llm.ToolCallResponse{}, f.CallToolErr
	}
	if f.callToolCount < len(f.Script) {
		r := f.Script[f.callToolCount]
		f.callToolCount++
		return r, nil
	}
	f.callToolCount++
	return f.ExhaustedResponse, nil
}

func (f *FakeClient) Stream(ctx context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	out := make(chan llm.StreamChunk)
	go func() {
		defer close(out)
		for _, text := range f.StreamChunks {
			select {
			case out <- llm.StreamChunk{Text: text}:
			case <-ctx.Done():
				return
			}
		}
		if f.StreamBlockForever {
			// 模拟慢上游：仅在 ctx 取消时退出，验证无 goroutine 泄漏。
			<-ctx.Done()
			select {
			case out <- llm.StreamChunk{Done: true, Err: ctx.Err()}:
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- llm.StreamChunk{Done: true}:
		case <-ctx.Done():
		}
	}()
	return out, nil
}
