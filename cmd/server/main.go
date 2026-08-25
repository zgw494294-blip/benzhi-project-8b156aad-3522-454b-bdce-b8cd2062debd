package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stone-restoration-trial:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		return err
	}
	if cfg.SelfCheck {
		return runSelfCheck(context.Background(), cfg)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := buildApplication(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.close()
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.Address, err)
	}
	fmt.Printf("石材修复试配验证工作台已启动：http://%s\n", cfg.Address)
	serveResult := make(chan error, 1)
	go func() { serveResult <- app.serve(listener) }()
	select {
	case err := <-serveResult:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := app.shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-serveResult
	}
}
