package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

func runSelfCheck(parent context.Context, cfg config) error {
	temporary, err := os.CreateTemp("", "stone-restoration-selfcheck-*.db")
	if err != nil {
		return err
	}
	path := temporary.Name()
	if err := temporary.Close(); err != nil {
		return err
	}
	defer os.Remove(path)
	defer os.Remove(path + "-wal")
	defer os.Remove(path + "-shm")
	cfg.DatabasePath = path
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	app, err := buildApplication(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.close()
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("自检监听 %s: %w", cfg.Address, err)
	}
	serveResult := make(chan error, 1)
	go func() { serveResult <- app.serve(listener) }()
	client := &http.Client{Timeout: 3 * time.Second}
	baseURL := "http://" + cfg.Address
	if err := waitForHealth(ctx, client, baseURL); err != nil {
		return err
	}
	if err := createSmokeCase(ctx, client, baseURL); err != nil {
		return err
	}
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	if err := app.shutdown(shutdownContext); err != nil {
		return err
	}
	select {
	case err := <-serveResult:
		return err
	case <-ctx.Done():
		return fmt.Errorf("等待服务退出超时: %w", ctx.Err())
	}
}

func waitForHealth(ctx context.Context, client *http.Client, baseURL string) error {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	var last error
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/healthz", nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Errorf("健康端点返回 %d", response.StatusCode)
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("健康检查未就绪: %v", last)
		case <-ticker.C:
		}
	}
}

func createSmokeCase(ctx context.Context, client *http.Client, baseURL string) error {
	payload := map[string]any{"idempotencyKey": "selfcheck-create-001", "actor": "自检程序", "siteName": "自检历史建筑", "buildingArea": "东立面基座", "stoneType": "花岗岩", "deteriorationSummary": "表面风化", "targetAppearance": "纹理与原石协调", "acceptanceThresholds": map[string]float64{"maxColorDifference": 3, "maxWaterAbsorption": 5, "minAdhesionStrength": 1}}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/restoration-cases", bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("创建自检任务: %w", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("创建自检任务返回 %d: %s", response.StatusCode, string(body))
	}
	return nil
}
