package run

import (
	"context"
	"testing"
	"time"
)

// AC-RUN-LOCK-001/002：同 project 串行、跨 project 并行。
func TestPerProjectLock(t *testing.T) {
	m := NewLockManager()

	// 项目 A 第一把锁占用。
	relA, err := m.Acquire(context.Background(), "A", time.Second)
	if err != nil {
		t.Fatalf("acquire A: %v", err)
	}

	// 项目 A 第二个 Run 无法立即获取（排队）。
	if _, ok := m.TryAcquire("A"); ok {
		t.Error("second run on project A must queue, but acquired immediately")
	}

	// 项目 B 立即并行获取。
	relB, ok := m.TryAcquire("B")
	if !ok {
		t.Error("project B must acquire in parallel")
	}
	relB()

	// 释放 A 后可再获取。
	relA()
	relA2, ok := m.TryAcquire("A")
	if !ok {
		t.Error("project A should be acquirable after release")
	}
	relA2()
}

// ARCH-RUN-LOCK-005：锁等待超时返回 ErrLockTimeout。
func TestLockTimeout(t *testing.T) {
	m := NewLockManager()
	rel, _ := m.Acquire(context.Background(), "A", time.Second)
	defer rel()

	_, err := m.Acquire(context.Background(), "A", 50*time.Millisecond)
	if err != ErrLockTimeout {
		t.Fatalf("want ErrLockTimeout, got %v", err)
	}
}

// 空 projectID（repo scope）不加锁。
func TestEmptyProjectNoLock(t *testing.T) {
	m := NewLockManager()
	r1, err := m.Acquire(context.Background(), "", time.Second)
	if err != nil {
		t.Fatalf("acquire empty: %v", err)
	}
	r2, ok := m.TryAcquire("")
	if !ok {
		t.Error("empty project must not block")
	}
	r1()
	r2()
}
