// Package config loads mlx-garde settings from ~/.config/mlx-garde/config.toml.
// Every field has a default, so the file is optional.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Server struct {
	Kind string `toml:"kind"` // omlx, ollama or lmstudio
	URL  string `toml:"url"`
	// KeyFile is read for the bearer token (oMLX keeps it in settings.json under auth.api_key).
	KeyFile string `toml:"key_file"`
	Enabled bool   `toml:"enabled"`
}

type Config struct {
	// MarginGB is kept free on top of what a job needs.
	MarginGB float64 `toml:"margin_gb"`
	// ModelFactor turns a model's size on disk into the memory it takes once loaded.
	ModelFactor float64 `toml:"model_factor"`
	// SwapAlertMB: a run that writes more than this to swap is flagged.
	SwapAlertMB float64 `toml:"swap_alert_mb"`
	// StateDir holds the lock, the queue and the journal.
	StateDir string   `toml:"state_dir"`
	Servers  []Server `toml:"servers"`
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		MarginGB:    6,
		ModelFactor: 1.3,
		SwapAlertMB: 256,
		StateDir:    filepath.Join(home, ".cache", "mlx-garde"),
		Servers: []Server{
			{Kind: "omlx", URL: "http://127.0.0.1:8000", KeyFile: filepath.Join(home, ".omlx", "settings.json"), Enabled: true},
			{Kind: "ollama", URL: "http://127.0.0.1:11434", Enabled: true},
			{Kind: "lmstudio", URL: "http://127.0.0.1:1234", Enabled: true},
		},
	}
}

func Path() string {
	if p := os.Getenv("MLX_GARDE_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "mlx-garde", "config.toml")
}

// Load reads path over the defaults. A missing file is not an error.
func Load(path string) (Config, error) {
	c := Default()
	if _, err := toml.DecodeFile(path, &c); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return c, err
	}
	c.StateDir = expand(c.StateDir)
	for i := range c.Servers {
		c.Servers[i].KeyFile = expand(c.Servers[i].KeyFile)
	}
	return c, nil
}

func expand(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}
	return p
}
