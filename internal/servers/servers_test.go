package servers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Odysseus-deep/mlx-garde/internal/config"
)

func TestLoadedModels(t *testing.T) {
	key := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(key, []byte(`{"auth":{"api_key":"k"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"models":[{"id":"qwen","loaded":true},{"id":"bge","loaded":false}]}`))
	})
	mux.HandleFunc("/api/ps", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"models":[{"name":"llama3:8b"}]}`))
	})
	mux.HandleFunc("/api/v0/models", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"gemma","state":"loaded"},{"id":"phi","state":"not-loaded"}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	got := LoadedModels(context.Background(), []config.Server{
		{Kind: "omlx", URL: srv.URL, KeyFile: key, Enabled: true},
		{Kind: "ollama", URL: srv.URL, Enabled: true},
		{Kind: "lmstudio", URL: srv.URL, Enabled: true},
		{Kind: "ollama", URL: srv.URL, Enabled: false},
		{Kind: "ollama", URL: "http://127.0.0.1:1", Enabled: true}, // down: reports nothing
	})
	want := []Loaded{{"omlx", "qwen"}, {"ollama", "llama3:8b"}, {"lmstudio", "gemma"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
