package main

import (
	"fmt"
	"net/http"

	"github.com/fguillen/dun-cli/internal/api"
	"github.com/fguillen/dun-cli/internal/auth"
	"github.com/fguillen/dun-cli/internal/config"
)

// session bundles the loaded config, credentials store, and an api.Client
// configured for the active base URL. Every Phase 2 subcommand calls
// loadSession at the top of its RunE.
type session struct {
	cfg    *config.Config
	store  *auth.Store
	client *api.Client
}

// loadSession reads ~/.dun/config.toml + ~/.dun/credentials and builds
// an api.Client. The Client uses a FileProvider so any in-process
// mutation of store.Current is picked up on the next API call without
// needing to rebuild the Client.
func loadSession() (*session, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	store, err := auth.LoadStore()
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}
	client, err := api.New(cfg.BaseURL, auth.NewFileProvider(store), &http.Client{})
	if err != nil {
		return nil, fmt.Errorf("build api client: %w", err)
	}
	return &session{cfg: cfg, store: store, client: client}, nil
}
