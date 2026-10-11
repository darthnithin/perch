package main

import (
	"fmt"
	"os"
	"path/filepath"

	"perch/internal/diagnostics"
	"perch/internal/domain"
)

func startDiagnostics(enabled, raw bool) (*diagnostics.Store, error) {
	if raw && !enabled {
		return nil, fmt.Errorf("--debug-raw requires --debug; raw scheduler responses may contain private job metadata")
	}
	if !enabled {
		return nil, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	mode := "metadata"
	if raw {
		mode = "raw"
	}
	store, err := diagnostics.New(filepath.Join(cache, "perch", "diagnostics", mode), version, raw)
	if err != nil {
		return nil, fmt.Errorf("cannot start diagnostics: %w", err)
	}
	fmt.Fprintf(os.Stderr, "perch: %s\n", domain.Label(store.Status()))
	return store, nil
}
func finishDiagnostics(store *diagnostics.Store) {
	if store == nil {
		return
	}
	if err := store.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "perch: diagnostic capture stopped: %s\n", domain.Label(err.Error()))
	}
}
