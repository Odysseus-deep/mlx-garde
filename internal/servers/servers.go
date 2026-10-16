// Package servers asks local inference servers which models they currently hold in memory.
// A server that is down or answers something unexpected simply reports nothing.
package servers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Odysseus-deep/mlx-garde/internal/config"
)

type Loaded struct {
	Server string // kind of server, e.g. "omlx"
	Model  string
}

// LoadedModels queries every enabled server in turn.
func LoadedModels(ctx context.Context, servers []config.Server) []Loaded {
	var all []Loaded
	for _, s := range servers {
		if !s.Enabled {
			continue
		}
		for _, m := range query(ctx, s) {
			all = append(all, Loaded{Server: s.Kind, Model: m})
		}
	}
	return all
}

func query(ctx context.Context, s config.Server) []string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	base := strings.TrimRight(s.URL, "/")
	switch s.Kind {
	case "omlx":
		var r struct {
			Models []struct {
				ID     string `json:"id"`
				Loaded bool   `json:"loaded"`
			} `json:"models"`
		}
		if getJSON(ctx, base+"/v1/models/status", omlxKey(s.KeyFile), &r) != nil {
			return nil
		}
		var ids []string
		for _, m := range r.Models {
			if m.Loaded {
				ids = append(ids, m.ID)
			}
		}
		return ids
	case "ollama":
		var r struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if getJSON(ctx, base+"/api/ps", "", &r) != nil {
			return nil
		}
		var ids []string
		for _, m := range r.Models {
			ids = append(ids, m.Name)
		}
		return ids
	case "lmstudio":
		var r struct {
			Data []struct {
				ID    string `json:"id"`
				State string `json:"state"`
			} `json:"data"`
		}
		if getJSON(ctx, base+"/api/v0/models", "", &r) != nil {
			return nil
		}
		var ids []string
		for _, m := range r.Data {
			if m.State == "loaded" {
				ids = append(ids, m.ID)
			}
		}
		return ids
	}
	return nil
}

// omlxKey reads auth.api_key from oMLX's settings.json.
func omlxKey(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var s struct {
		Auth struct {
			APIKey string `json:"api_key"`
		} `json:"auth"`
	}
	if json.Unmarshal(b, &s) != nil {
		return ""
	}
	return s.Auth.APIKey
}

func getJSON(ctx context.Context, url, bearer string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &statusError{resp.StatusCode}
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

type statusError struct{ code int }

func (e *statusError) Error() string { return http.StatusText(e.code) }
