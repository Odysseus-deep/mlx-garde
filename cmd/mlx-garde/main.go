// mlx-garde runs heavy local LLM jobs one at a time, refuses those that would not fit in RAM,
// and reports whether a run made the Mac swap.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Odysseus-deep/mlx-garde/internal/config"
	"github.com/Odysseus-deep/mlx-garde/internal/guard"
	"github.com/Odysseus-deep/mlx-garde/internal/lock"
	"github.com/Odysseus-deep/mlx-garde/internal/memory"
	"github.com/Odysseus-deep/mlx-garde/internal/servers"
	"github.com/Odysseus-deep/mlx-garde/internal/ui"
)

var version = "dev"

const usage = `usage:
  mlx-garde [--model <dir> | --need <GB>] [--no-wait] [--] <command…>
  mlx-garde status [--watch]

  --model <dir>   the command loads this model itself: refuse if a local server already holds a model,
                  or if available memory < size × model_factor + margin
  --need <GB>     the command loads models on its own (several, or not from one folder):
                  refuse if available memory < need + margin
  --no-wait       fail with exit code 75 instead of queueing behind a running job
  status          lock, queue, loaded models, memory and swap; --watch for the live view

config: ~/.config/mlx-garde/config.toml (or $MLX_GARDE_CONFIG)
`

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	cfg, err := config.Load(config.Path())
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlx-garde: config: %v\n", err)
		return 2
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mlx-garde: %v\n", err)
		return 1
	}
	if len(args) > 0 && args[0] == "status" {
		return status(cfg, args[1:])
	}

	fs := flag.NewFlagSet("mlx-garde", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	modelDir := fs.String("model", "", "")
	needGB := fs.Float64("need", 0, "")
	noWait := fs.Bool("no-wait", false, "")
	showVersion := fs.Bool("version", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Println("mlx-garde", version)
		return 0
	}
	cmd := fs.Args()
	if len(cmd) == 0 || (*modelDir != "" && *needGB > 0) {
		fs.Usage()
		return 2
	}

	// 1. Queue
	l := lock.New(cfg.StateDir)
	wd, _ := os.Getwd()
	job := lock.Job{PID: os.Getpid(), Dir: wd, Command: strings.Join(cmd, " "), Since: time.Now()}
	if *noWait {
		if err := l.TryAcquire(job); errors.Is(err, lock.ErrBusy) {
			fmt.Fprintf(os.Stderr, "mlx-garde: refused (--no-wait), a job is running: %s\n", holderLine(l.Holder()))
			return guard.ExitRefused
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "mlx-garde: lock: %v\n", err)
			return 1
		}
	} else {
		err := l.Acquire(job, 2*time.Second, func(h *lock.Job) {
			fmt.Fprintf(os.Stderr, "mlx-garde: waiting for: %s\n", holderLine(h))
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "mlx-garde: lock: %v\n", err)
			return 1
		}
	}
	defer l.Release()

	// 2. Memory, only when the job loads a model itself
	if *modelDir != "" || *needGB > 0 {
		var size uint64
		if *modelDir != "" {
			if size, err = guard.ModelSize(*modelDir); err != nil {
				fmt.Fprintf(os.Stderr, "mlx-garde: model: %v\n", err)
				return 2
			}
		}
		need := guard.Need(size, cfg.ModelFactor, *needGB, cfg.MarginGB)
		st, err := memory.Read()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mlx-garde: %v\n", err)
			return 1
		}
		loaded := servers.LoadedModels(context.Background(), cfg.Servers)
		if err := guard.Check(need, st, loaded, *modelDir != ""); err != nil {
			fmt.Fprintf(os.Stderr, "mlx-garde: refused, %v. Biggest memory users:\n", err)
			for _, p := range guard.TopProcesses(5) {
				fmt.Fprintf(os.Stderr, "  %5.1f GB  %s\n", memory.GB(p.RSS), p.Name)
			}
			return guard.ExitRefused
		}
	}

	// 3. Run, then report swap
	before, _ := memory.Read()
	guard.Journal(cfg.StateDir, "START %s : %s (available %.1f GB, swap %.0f MB)", wd, job.Command,
		memory.GB(before.AvailableBytes), memory.MB(before.SwapUsed))
	t0 := time.Now()
	rc := guard.Run(cmd)
	after, _ := memory.Read()
	rep := guard.Report(before, after, cfg.SwapAlertMB)
	guard.Journal(cfg.StateDir, "END rc=%d %s %s : %s", rc, time.Since(t0).Round(time.Second), rep, job.Command)
	fmt.Fprintf(os.Stderr, "mlx-garde: %s\n", rep)
	return rc
}

func status(cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	watch := fs.Bool("watch", false, "")
	if fs.Parse(args) != nil {
		return 2
	}
	if *watch {
		if err := ui.Watch(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "mlx-garde: %v\n", err)
			return 1
		}
		return 0
	}
	ui.Print(ui.Take(cfg, true))
	return 0
}

func holderLine(h *lock.Job) string {
	if h == nil || h.PID == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%s (pid %d, since %s)", h.Command, h.PID, h.Since.Format("15:04"))
}
