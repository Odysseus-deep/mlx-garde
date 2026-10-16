// Package ui renders the status, once as plain text or live in the terminal.
package ui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Odysseus-deep/mlx-garde/internal/config"
	"github.com/Odysseus-deep/mlx-garde/internal/guard"
	"github.com/Odysseus-deep/mlx-garde/internal/lock"
	"github.com/Odysseus-deep/mlx-garde/internal/memory"
	"github.com/Odysseus-deep/mlx-garde/internal/servers"
)

// Snapshot is everything the status shows at one instant.
type Snapshot struct {
	At     time.Time
	Mem    memory.Stats
	Holder *lock.Job
	Queue  []lock.Job
	Loaded []servers.Loaded
	Top    []guard.Process
	Recent []string
}

func Take(cfg config.Config, withServers bool) Snapshot {
	l := lock.New(cfg.StateDir)
	s := Snapshot{At: time.Now(), Holder: l.Holder(), Queue: l.Queue(), Top: guard.TopProcesses(5), Recent: tail(filepath.Join(cfg.StateDir, "journal.log"), 4)}
	s.Mem, _ = memory.Read()
	if withServers {
		s.Loaded = servers.LoadedModels(context.Background(), cfg.Servers)
	}
	return s
}

// Print writes the status once, for scripts and quick checks.
func Print(s Snapshot) {
	if s.Holder == nil {
		fmt.Println("lock: free")
	} else {
		fmt.Printf("lock: held by %s\n", jobLine(*s.Holder, s.At))
	}
	for _, j := range s.Queue {
		fmt.Printf("waiting: %s\n", jobLine(j, s.At))
	}
	fmt.Printf("loaded models: %s\n", loadedLine(s.Loaded))
	fmt.Printf("memory: %.1f GB available of %.0f GB; swap used %.0f MB; swapouts %d\n",
		memory.GB(s.Mem.AvailableBytes), memory.GB(s.Mem.TotalBytes), memory.MB(s.Mem.SwapUsed), s.Mem.Swapouts)
	for _, p := range s.Top {
		fmt.Printf("  %5.1f GB  %s\n", memory.GB(p.RSS), p.Name)
	}
}

func jobLine(j lock.Job, now time.Time) string {
	if j.PID == 0 {
		return "(starting)"
	}
	return fmt.Sprintf("%s  (pid %d, %s, in %s)", j.Command, j.PID, now.Sub(j.Since).Round(time.Second), tilde(j.Dir))
}

func tilde(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return dir
	}
	home = filepath.Clean(home)
	if dir == home || strings.HasPrefix(dir, home+"/") {
		return "~" + dir[len(home):]
	}
	return dir
}

func loadedLine(l []servers.Loaded) string {
	if len(l) == 0 {
		return "none"
	}
	var parts []string
	for _, m := range l {
		parts = append(parts, m.Server+":"+m.Model)
	}
	return strings.Join(parts, ", ")
}

func tail(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines
}

// Watch runs the live view until q or Ctrl-C.
func Watch(cfg config.Config) error {
	_, err := tea.NewProgram(model{cfg: cfg}, tea.WithAltScreen()).Run()
	return err
}

const history = 60

type tick time.Time
type loadedMsg []servers.Loaded

type model struct {
	cfg     config.Config
	snap    Snapshot
	loaded  []servers.Loaded
	swapMB  []float64 // MB written to swap during each tick
	width   int
	lastOut uint64
	started bool
}

func (m model) Init() tea.Cmd { return tea.Batch(doTick(0), fetchLoaded(m.cfg)) }

func doTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tick(t) })
}

func fetchLoaded(cfg config.Config) tea.Cmd {
	return func() tea.Msg { return loadedMsg(servers.LoadedModels(context.Background(), cfg.Servers)) }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case loadedMsg:
		m.loaded = msg
		return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return fetchLoaded(m.cfg)() })
	case tick:
		m.snap = Take(m.cfg, false)
		if m.started && m.snap.Mem.Swapouts >= m.lastOut {
			m.swapMB = append(m.swapMB, memory.MB((m.snap.Mem.Swapouts-m.lastOut)*m.snap.Mem.PageSize))
			if len(m.swapMB) > history {
				m.swapMB = m.swapMB[1:]
			}
		}
		m.lastOut, m.started = m.snap.Mem.Swapouts, true
		return m, doTick(time.Second)
	}
	return m, nil
}

var (
	title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	label = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(10)
	ok    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	warn  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	bad   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	dim   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	box   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)
)

func (m model) View() string {
	s := m.snap
	if s.At.IsZero() {
		return "reading…"
	}
	w := m.width - 6 // border and padding
	if w < 40 {
		w = 74
	}
	barW := w - 34
	if barW < 10 {
		barW = 10
	}
	var b strings.Builder
	b.WriteString(title.Render("mlx-garde") + dim.Render("  "+s.At.Format("15:04:05")+"  q to quit") + "\n\n")

	total, avail := memory.GB(s.Mem.TotalBytes), memory.GB(s.Mem.AvailableBytes)
	used := 0.0
	if total > 0 {
		used = 1 - avail/total
	}
	style := ok
	switch {
	case avail < m.cfg.MarginGB:
		style = bad
	case avail < 2*m.cfg.MarginGB:
		style = warn
	}
	b.WriteString(label.Render("memory") + style.Render(bar(used, barW)) + fmt.Sprintf("  %.1f GB free of %.0f\n", avail, total))

	last := 0.0
	if n := len(m.swapMB); n > 0 {
		last = m.swapMB[n-1]
	}
	b.WriteString(label.Render("swap") + spark(m.swapMB, barW) +
		fmt.Sprintf("  %.0f MB used, %.0f MB/s\n\n", memory.MB(s.Mem.SwapUsed), last))

	if s.Holder == nil {
		b.WriteString(label.Render("job") + ok.Render("none, the lock is free") + "\n")
	} else {
		b.WriteString(label.Render("job") + warn.Render(clip(jobLine(*s.Holder, s.At), w-10)) + "\n")
	}
	for i, j := range s.Queue {
		b.WriteString(label.Render(fmt.Sprintf("queue %d", i+1)) + clip(jobLine(j, s.At), w-10) + "\n")
	}
	b.WriteString(label.Render("loaded") + clip(loadedLine(m.loaded), w-10) + "\n\n")

	for _, p := range s.Top {
		b.WriteString(dim.Render(fmt.Sprintf("%10.1f GB  ", memory.GB(p.RSS))) + clip(p.Name, w-16) + "\n")
	}
	if len(s.Recent) > 0 {
		b.WriteString("\n")
		for _, l := range s.Recent {
			b.WriteString(dim.Render(clip(tildeAll(l), w)) + "\n")
		}
	}
	return box.Render(strings.TrimRight(b.String(), "\n"))
}

func bar(frac float64, w int) string {
	n := int(frac*float64(w) + 0.5)
	if n > w {
		n = w
	}
	if n < 0 {
		n = 0
	}
	return strings.Repeat("█", n) + dim.Render(strings.Repeat("░", w-n))
}

// spark draws the last w samples: a dim baseline when nothing was swapped, red blocks scaled to the worst second.
func spark(v []float64, w int) string {
	blocks := []rune("▁▂▃▄▅▆▇█")
	if len(v) > w {
		v = v[len(v)-w:]
	}
	max := 0.0
	for _, x := range v {
		if x > max {
			max = x
		}
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", w-len(v)))
	for _, x := range v {
		if x <= 0 {
			b.WriteString(dim.Render("▁"))
			continue
		}
		b.WriteString(bad.Render(string(blocks[1+int(x/max*float64(len(blocks)-2))])))
	}
	return b.String()
}

func clip(s string, w int) string {
	r := []rune(s)
	if w > 1 && len(r) > w {
		return string(r[:w-1]) + "…"
	}
	return s
}

// tildeAll shortens every occurrence of the home directory in a journal line.
func tildeAll(line string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return line
	}
	return strings.ReplaceAll(line, filepath.Clean(home), "~")
}
