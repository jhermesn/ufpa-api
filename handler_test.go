package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInfoReturnsTaskIdentity(t *testing.T) {
	handler := newHandler(TaskInfo{TaskID: "abc123", AZ: "us-east-1a"})
	req := httptest.NewRequest(http.MethodGet, "/v1/info", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var got infoResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := infoResponse{App: "ufpa-api", Version: "dev", TaskID: "abc123", AZ: "us-east-1a"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestInfoRejectsPost(t *testing.T) {
	handler := newHandler(localTask)
	req := httptest.NewRequest(http.MethodPost, "/v1/info", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHealthzReturnsOK(t *testing.T) {
	handler := newHandler(localTask)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLoadTaskInfoReadsMetadataEndpoint(t *testing.T) {
	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"TaskARN":"arn:aws:ecs:us-east-1:111122223333:task/demo/f00ba7","AvailabilityZone":"us-east-1b"}`))
	}))
	defer metadata.Close()

	got := loadTaskInfo(context.Background(), metadata.URL)

	want := TaskInfo{TaskID: "f00ba7", AZ: "us-east-1b"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadTaskInfoFallsBackWhenEndpointFails(t *testing.T) {
	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer metadata.Close()

	got := loadTaskInfo(context.Background(), metadata.URL)

	if got != localTask {
		t.Errorf("got %+v, want %+v", got, localTask)
	}
}

func TestLoadTaskInfoIsLocalOutsideECS(t *testing.T) {
	got := loadTaskInfo(context.Background(), "")

	if got != localTask {
		t.Errorf("got %+v, want %+v", got, localTask)
	}
}

func TestListenAddrFallsBackOnInvalidPort(t *testing.T) {
	got := listenAddr("not-a-port")

	if got != defaultAddr {
		t.Errorf("got %q, want %q", got, defaultAddr)
	}
}
