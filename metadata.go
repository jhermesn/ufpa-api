package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	metadataTimeout  = 2 * time.Second
	maxMetadataBytes = 1 << 20
)

type TaskInfo struct {
	TaskID string
	AZ     string
}

type taskMetadata struct {
	TaskARN          string `json:"TaskARN"`
	AvailabilityZone string `json:"AvailabilityZone"`
}

var localTask = TaskInfo{TaskID: "local", AZ: "local"}

func loadTaskInfo(ctx context.Context, metadataURI string) TaskInfo {
	if metadataURI == "" {
		return localTask
	}
	meta, err := fetchTaskMetadata(ctx, metadataURI+"/task")
	if err != nil {
		slog.Warn("task metadata unavailable, using local identity", "error", err)
		return localTask
	}
	return TaskInfo{TaskID: shortTaskID(meta.TaskARN), AZ: meta.AvailabilityZone}
}

func fetchTaskMetadata(ctx context.Context, url string) (taskMetadata, error) {
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return taskMetadata{}, fmt.Errorf("build metadata request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return taskMetadata{}, fmt.Errorf("call metadata endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return taskMetadata{}, fmt.Errorf("metadata endpoint returned status %d", resp.StatusCode)
	}
	return decodeTaskMetadata(resp.Body)
}

func decodeTaskMetadata(body io.Reader) (taskMetadata, error) {
	var meta taskMetadata
	if err := json.NewDecoder(io.LimitReader(body, maxMetadataBytes)).Decode(&meta); err != nil {
		return taskMetadata{}, fmt.Errorf("decode metadata: %w", err)
	}
	return meta, nil
}

func shortTaskID(taskARN string) string {
	return taskARN[strings.LastIndex(taskARN, "/")+1:]
}
