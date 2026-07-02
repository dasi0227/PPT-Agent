// Package run 是 Run 生命周期外壳：状态机 + SSE 事件扇出 + 控制输入队列(HITL) + 每 project 执行锁。
// 驱动 harness（认知引擎），自身不含业务逻辑（ARCH-RUNTIME）。
package run

import (
	"context"
	"sync"
	"time"
)

// LockManager 实现每 project 一把执行锁：同 project 串行、跨 project 并行（ARCH-RUN-LOCK-001/002）。
type LockManager struct {
	mu    sync.Mutex
	locks map[string]*projectLock
}

type projectLock struct {
	ch chan struct{} // 容量 1 的信号量
}

func NewLockManager() *LockManager {
	return &LockManager{locks: map[string]*projectLock{}}
}

func (m *LockManager) lockFor(projectID string) *projectLock {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.locks[projectID]
	if !ok {
		l = &projectLock{ch: make(chan struct{}, 1)}
		m.locks[projectID] = l
	}
	return l
}

// Acquire 获取 project 锁；锁等待有上限，超时返回 ErrLockTimeout（ARCH-RUN-LOCK-005）。
// projectID 为空（如 repo scope 无 project）时不加锁，直接返回空释放函数。
func (m *LockManager) Acquire(ctx context.Context, projectID string, timeout time.Duration) (func(), error) {
	if projectID == "" {
		return func() {}, nil
	}
	l := m.lockFor(projectID)

	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}

	select {
	case l.ch <- struct{}{}:
		return func() { <-l.ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer:
		return nil, ErrLockTimeout
	}
}

// TryAcquire 非阻塞获取；用于测试断言「同 project 第二个 Run 排队」。
func (m *LockManager) TryAcquire(projectID string) (func(), bool) {
	if projectID == "" {
		return func() {}, true
	}
	l := m.lockFor(projectID)
	select {
	case l.ch <- struct{}{}:
		return func() { <-l.ch }, true
	default:
		return nil, false
	}
}
