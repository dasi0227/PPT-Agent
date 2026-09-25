package workrootlock

import "testing"

func TestSingleWriterAndRelease(t *testing.T) {
	root := t.TempDir()
	release, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if second, err := Acquire(root); err == nil {
		second()
		t.Fatal("a second writer acquired the same work directory")
	}
	release()
	third, err := Acquire(root)
	if err != nil {
		t.Fatal("lock did not release", err)
	}
	defer third()
	// A repeated cleanup must not release a later holder's lock.
	release()
	if fourth, err := Acquire(root); err == nil {
		fourth()
		t.Fatal("repeated cleanup released a different holder")
	}
}
