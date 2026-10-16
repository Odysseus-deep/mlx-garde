package ui

import (
	"os"
	"testing"
	"time"

	"github.com/Odysseus-deep/mlx-garde/internal/config"
	"github.com/Odysseus-deep/mlx-garde/internal/guard"
	"github.com/Odysseus-deep/mlx-garde/internal/lock"
	"github.com/Odysseus-deep/mlx-garde/internal/memory"
	"github.com/Odysseus-deep/mlx-garde/internal/servers"
)

// TestView renders a fixed snapshot; run with -v to look at it.
func TestView(t *testing.T) {
	now := time.Date(2026, 10, 16, 23, 50, 0, 0, time.UTC)
	m := model{
		cfg:   config.Default(),
		width: 90,
		snap: Snapshot{
			At:     now,
			Mem:    memory.Stats{TotalBytes: 64 << 30, AvailableBytes: 9 << 30, SwapUsed: 1200 << 20},
			Holder: &lock.Job{PID: 4242, Dir: "~/code/phemios", Command: "python transcribe.py --language fr salon.wav", Since: now.Add(-7 * time.Minute)},
			Queue:  []lock.Job{{PID: 4250, Dir: "~/code/histos", Command: "python eval.py --sample 20", Since: now.Add(-2 * time.Minute)}},
			Top:    []guard.Process{{RSS: 31 << 30, Name: "python3.12"}, {RSS: 9 << 30, Name: "OrbStack Helper"}},
			Recent: []string{"[2026-10-16 23:43:07] END rc=0 12m4s swap OK (0 MB written, swap 0 MB → 0 MB) : python embed.py"},
		},
		loaded: []servers.Loaded{{Server: "omlx", Model: "Qwen3.8-27B-4bit"}},
		swapMB: []float64{0, 0, 0, 0, 3, 40, 120, 80, 10, 0, 0, 0},
	}
	v := m.View()
	if v == "" {
		t.Fatal("empty view")
	}
	if os.Getenv("SHOW") != "" {
		t.Log("\n" + v)
	}
}
