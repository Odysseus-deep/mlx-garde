package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingFileGivesDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil || c.MarginGB != 6 || len(c.Servers) != 3 {
		t.Fatalf("%+v, %v", c, err)
	}
}

func TestFileOverridesAndExpandsHome(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte("margin_gb = 4\nstate_dir = \"~/x\"\n[[servers]]\nkind = \"ollama\"\nurl = \"http://h\"\nenabled = true\n"), 0o644)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	if c.MarginGB != 4 || c.ModelFactor != 1.3 || c.StateDir != filepath.Join(home, "x") {
		t.Errorf("%+v", c)
	}
	if len(c.Servers) != 1 || c.Servers[0].Kind != "ollama" {
		t.Errorf("servers should replace the defaults: %+v", c.Servers)
	}
}
