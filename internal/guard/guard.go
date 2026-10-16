// Package guard holds the decisions: does a job fit in memory, did a run swap, and the run itself.
package guard

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Odysseus-deep/mlx-garde/internal/memory"
	"github.com/Odysseus-deep/mlx-garde/internal/servers"
)

// ExitRefused is EX_TEMPFAIL: the job was not run, retrying later may work.
const ExitRefused = 75

// ModelSize sums the files under dir, following symlinks (Hugging Face caches are made of them).
func ModelSize(dir string) (uint64, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return 0, err
	}
	var total uint64
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += uint64(info.Size())
		}
		return nil
	})
	return total, err
}

// Need is the memory a job asks for, margin included.
func Need(modelBytes uint64, factor, needGB, marginGB float64) float64 {
	if modelBytes > 0 {
		return memory.GB(modelBytes)*factor + marginGB
	}
	return needGB + marginGB
}

// Check refuses a job that would not fit. loaded is what local servers already hold.
func Check(needGB float64, st memory.Stats, loaded []servers.Loaded, refuseIfLoaded bool) error {
	if refuseIfLoaded && len(loaded) > 0 {
		return fmt.Errorf("a local server already holds %s; go through its API or unload it first", describe(loaded))
	}
	if avail := memory.GB(st.AvailableBytes); needGB > avail {
		msg := fmt.Sprintf("needs about %.1f GB, %.1f GB available", needGB, avail)
		if len(loaded) > 0 {
			msg += fmt.Sprintf("; unloading %s would free some", describe(loaded))
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func describe(loaded []servers.Loaded) string {
	var parts []string
	for _, l := range loaded {
		parts = append(parts, l.Server+":"+l.Model)
	}
	return strings.Join(parts, ", ")
}

// SwapReport compares memory before and after a run.
type SwapReport struct {
	WrittenMB     float64
	Before, After uint64 // swap used, bytes
	Swapped       bool
}

func Report(before, after memory.Stats, alertMB float64) SwapReport {
	var pages uint64
	if after.Swapouts > before.Swapouts {
		pages = after.Swapouts - before.Swapouts
	}
	mb := memory.MB(pages * after.PageSize)
	return SwapReport{WrittenMB: mb, Before: before.SwapUsed, After: after.SwapUsed, Swapped: mb > alertMB}
}

func (r SwapReport) String() string {
	swap := fmt.Sprintf("swap %.0f MB → %.0f MB", memory.MB(r.Before), memory.MB(r.After))
	if r.Swapped {
		return fmt.Sprintf("⚠ the run swapped: %.0f MB written to disk (%s)", r.WrittenMB, swap)
	}
	return fmt.Sprintf("swap OK (%.0f MB written, %s)", r.WrittenMB, swap)
}

// Run starts the command, forwards termination signals to it and returns its exit code.
// Ctrl-C and Ctrl-\ already reach the child through the terminal's process group: they are only caught
// here, so mlx-garde outlives the child and releases the lock. SIGTERM and SIGHUP, sent to mlx-garde
// alone, are passed on.
func Run(args []string) int {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Notify, not Ignore: an ignored signal stays ignored in the child after exec.
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "mlx-garde: %v\n", err)
		return 127
	}
	go func() {
		for s := range sigs {
			if s == syscall.SIGTERM || s == syscall.SIGHUP {
				cmd.Process.Signal(s)
			}
		}
	}()
	err := cmd.Wait()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return ee.ExitCode()
	}
	return 1
}

// Process is one entry of the biggest memory users.
type Process struct {
	RSS  uint64
	Name string
}

// TopProcesses lists the n processes with the largest resident memory.
func TopProcesses(n int) []Process {
	out, err := exec.Command("ps", "-axo", "rss=,comm=").Output()
	if err != nil {
		return nil
	}
	var ps []Process
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		kb, err := strconv.ParseUint(f[0], 10, 64)
		if err != nil {
			continue
		}
		ps = append(ps, Process{RSS: kb * 1024, Name: filepath.Base(strings.Join(f[1:], " "))})
	}
	sort.Slice(ps, func(a, b int) bool { return ps[a].RSS > ps[b].RSS })
	if len(ps) > n {
		ps = ps[:n]
	}
	return ps
}

// Journal appends one line to the run log.
func Journal(dir, format string, a ...any) {
	f, err := os.OpenFile(filepath.Join(dir, "journal.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, a...))
}
