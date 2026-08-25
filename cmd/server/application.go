package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"stone-restoration-trial/internal/httpapi"
	"stone-restoration-trial/internal/store"
	"stone-restoration-trial/internal/workflow"
	"time"
)

type application struct {
	config config
	store  *store.SQLiteStore
	server *http.Server
}

func buildApplication(ctx context.Context, cfg config) (*application, error) {
	if cfg.DatabasePath != ":memory:" {
		directory := filepath.Dir(cfg.DatabasePath)
		if directory != "." {
			if err := os.MkdirAll(directory, 0o750); err != nil {
				return nil, fmt.Errorf("创建数据库目录: %w", err)
			}
		}
	}
	repository, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	service := workflow.New(repository)
	api := httpapi.New(service)
	server := &http.Server{Addr: cfg.Address, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	return &application{config: cfg, store: repository, server: server}, nil
}

func (a *application) close() error { return a.store.Close() }

func (a *application) serve(listener net.Listener) error {
	err := a.server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *application) shutdown(ctx context.Context) error { return a.server.Shutdown(ctx) }
