package guard

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Odysseus-deep/mlx-garde/internal/memory"
	"github.com/Odysseus-deep/mlx-garde/internal/servers"
)

const gb = 1 << 30

func TestNeed(t *testing.T) {
	if got := Need(10*gb, 1.3, 0, 6); math.Abs(got-19) > 1e-9 {
		t.Errorf("model need = %v, want 19", got)
	}
	if got := Need(0, 1.3, 20, 6); got != 26 {
		t.Errorf("declared need = %v, want 26", got)
	}
}

func TestCheck(t *testing.T) {
	st := memory.Stats{AvailableBytes: 30 * gb}
	if err := Check(25, st, nil, true); err != nil {
		t.Errorf("fits: %v", err)
	}
	if err := Check(31, st, nil, true); err == nil {
		t.Error("too big: no error")
	}
	loaded := []servers.Loaded{{Server: "omlx", Model: "qwen"}}
	if err := Check(1, st, loaded, true); err == nil || !strings.Contains(err.Error(), "omlx:qwen") {
		t.Errorf("model already loaded: %v", err)
	}
	if err := Check(1, st, loaded, false); err != nil {
		t.Errorf("--need ignores loaded models when memory suffices: %v", err)
	}
}

func TestReportCountsSwapoutsOnly(t *testing.T) {
	before := memory.Stats{Swapouts: 100, PageSize: 16384}
	after := memory.Stats{Swapouts: 100 + 20000, PageSize: 16384} // 20000 × 16 KB ≈ 312 MB
	r := Report(before, after, 256)
	if !r.Swapped || math.Abs(r.WrittenMB-312.5) > 0.1 {
		t.Errorf("report = %+v", r)
	}
	if r := Report(before, before, 256); r.Swapped || r.WrittenMB != 0 {
		t.Errorf("no swap: %+v", r)
	}
}

func TestModelSizeFollowsSymlinks(t *testing.T) {
	blobs, snap := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(blobs, "a"), make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snap, "config.json"), make([]byte, 24), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(blobs, "a"), filepath.Join(snap, "model.safetensors")); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "model")
	if err := os.Symlink(snap, link); err != nil {
		t.Fatal(err)
	}
	if got, err := ModelSize(link); err != nil || got != 1024 {
		t.Errorf("size = %d, %v; want 1024", got, err)
	}
}

func TestRunExitCode(t *testing.T) {
	if rc := Run([]string{"sh", "-c", "exit 3"}); rc != 3 {
		t.Errorf("rc = %d, want 3", rc)
	}
	if rc := Run([]string{"sh", "-c", "kill -TERM $$"}); rc != 128+15 {
		t.Errorf("rc = %d, want 143", rc)
	}
	if rc := Run([]string{"/nonexistent"}); rc != 127 {
		t.Errorf("rc = %d, want 127", rc)
	}
}
