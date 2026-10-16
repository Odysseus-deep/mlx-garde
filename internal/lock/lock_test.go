package lock

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestBusyThenFree(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)
	job := Job{PID: os.Getpid(), Command: "first", Since: time.Now()}
	if err := a.TryAcquire(job); err != nil {
		t.Fatal(err)
	}
	if h := a.Holder(); h == nil || h.Command != "first" {
		t.Fatalf("holder = %+v, want first", h)
	}
	b := New(dir)
	if err := b.TryAcquire(Job{PID: 1, Command: "second"}); !errors.Is(err, ErrBusy) {
		t.Fatalf("second acquire: %v, want ErrBusy", err)
	}
	a.Release()
	if h := b.Holder(); h != nil {
		t.Fatalf("holder after release = %+v, want nil", h)
	}
	if err := b.TryAcquire(Job{PID: 1, Command: "second"}); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	b.Release()
}

func TestWaiterIsQueuedThenServed(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)
	if err := a.TryAcquire(Job{PID: os.Getpid(), Command: "running"}); err != nil {
		t.Fatal(err)
	}
	waited := make(chan *Job, 1)
	done := make(chan error, 1)
	b := New(dir)
	go func() {
		done <- b.Acquire(Job{PID: os.Getpid(), Command: "waiting", Since: time.Now()}, 10*time.Millisecond,
			func(h *Job) { waited <- h })
	}()
	if h := <-waited; h == nil || h.Command != "running" {
		t.Fatalf("onWait holder = %+v", h)
	}
	if q := a.Queue(); len(q) != 1 || q[0].Command != "waiting" {
		t.Fatalf("queue = %+v", q)
	}
	a.Release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if q := b.Queue(); len(q) != 0 {
		t.Fatalf("queue after acquire = %+v", q)
	}
	b.Release()
}

func TestDeadWaiterDropped(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	if err := os.MkdirAll(l.queueDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// pid 0x7ffffffe is never a live process
	if err := writeJSON(l.queueEntry(0x7ffffffe), Job{PID: 0x7ffffffe}); err != nil {
		t.Fatal(err)
	}
	if q := l.Queue(); len(q) != 0 {
		t.Fatalf("queue = %+v, want empty", q)
	}
}
