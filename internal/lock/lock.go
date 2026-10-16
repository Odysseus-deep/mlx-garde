// Package lock serializes heavy jobs with flock(2): the kernel releases the lock when the holder dies,
// so a killed job never leaves the queue stuck. Waiting jobs register in queue/ so status can show them.
package lock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Job describes a holder or a waiter.
type Job struct {
	PID     int       `json:"pid"`
	Dir     string    `json:"dir"`
	Command string    `json:"command"`
	Since   time.Time `json:"since"`
}

type Lock struct {
	dir string
	f   *os.File
}

var ErrBusy = errors.New("another job holds the lock")

func New(stateDir string) *Lock { return &Lock{dir: stateDir} }

func (l *Lock) lockPath() string   { return filepath.Join(l.dir, "lock") }
func (l *Lock) holderPath() string { return filepath.Join(l.dir, "holder.json") }
func (l *Lock) queueDir() string   { return filepath.Join(l.dir, "queue") }

// TryAcquire takes the lock without waiting, or returns ErrBusy.
func (l *Lock) TryAcquire(job Job) error {
	f, err := l.flock()
	if err != nil {
		return err
	}
	l.f = f
	return writeJSON(l.holderPath(), job)
}

func (l *Lock) flock() (*os.File, error) {
	if err := os.MkdirAll(l.queueDir(), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(l.lockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return f, nil
}

// Acquire waits in the queue until the lock is free. onWait is called once, with the current holder.
func (l *Lock) Acquire(job Job, poll time.Duration, onWait func(holder *Job)) error {
	announced := false
	defer os.Remove(l.queueEntry(job.PID))
	for {
		err := l.TryAcquire(job)
		if !errors.Is(err, ErrBusy) {
			return err
		}
		if !announced {
			announced = true
			if err := writeJSON(l.queueEntry(job.PID), job); err != nil {
				return err
			}
			if onWait != nil {
				onWait(l.Holder())
			}
		}
		time.Sleep(poll)
	}
}

func (l *Lock) Release() {
	if l.f == nil {
		return
	}
	os.Remove(l.holderPath())
	unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	l.f.Close()
	l.f = nil
}

func (l *Lock) queueEntry(pid int) string {
	return filepath.Join(l.queueDir(), strconv.Itoa(pid)+".json")
}

// Holder returns the job holding the lock, or nil if it is free.
func (l *Lock) Holder() *Job {
	if f, err := l.flock(); err == nil {
		os.Remove(l.holderPath()) // left behind by a holder that was killed
		f.Close()
		return nil
	}
	var j Job
	if readJSON(l.holderPath(), &j) != nil {
		return &Job{}
	}
	return &j
}

// Queue lists waiting jobs, oldest first, dropping entries whose process is gone.
func (l *Lock) Queue() []Job {
	entries, _ := os.ReadDir(l.queueDir())
	var jobs []Job
	for _, e := range entries {
		p := filepath.Join(l.queueDir(), e.Name())
		var j Job
		if readJSON(p, &j) != nil || !alive(j.PID) {
			os.Remove(p)
			continue
		}
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(a, b int) bool { return jobs[a].Since.Before(jobs[b].Since) })
	return jobs
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func writeJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
