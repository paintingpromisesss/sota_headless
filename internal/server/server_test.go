package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sota-headless/internal/config"
	"sota-headless/internal/controller"
	"sota-headless/internal/logger"
)

func TestServerHealthAndRequestLogging(t *testing.T) {
	var logBuf bytes.Buffer
	logger.Setup("info", "text", &logBuf)

	tempDir := t.TempDir()
	cfg := config.Config{
		BaseDir:   tempDir,
		AccessKey: "test-access-key",
		APIBases:  []string{"https://example.test/api/v1"},
	}

	ctrl, err := controller.New(cfg)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	srv := Server{Controller: ctrl}
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, `path=/health`) {
		t.Errorf("expected path=/health in log, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `status=200`) {
		t.Errorf("expected status=200 in log, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `method=GET`) {
		t.Errorf("expected method=GET in log, got: %s", logOutput)
	}
}

func TestServerNotFoundLogging(t *testing.T) {
	var logBuf bytes.Buffer
	logger.Setup("info", "text", &logBuf)

	tempDir := t.TempDir()
	cfg := config.Config{
		BaseDir:   tempDir,
		AccessKey: "test-access-key",
		APIBases:  []string{"https://example.test/api/v1"},
	}

	ctrl, err := controller.New(cfg)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	srv := Server{Controller: ctrl}
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/non-existent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, `level=WARN`) {
		t.Errorf("expected level=WARN in log for 404, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `path=/non-existent`) {
		t.Errorf("expected path=/non-existent in log, got: %s", logOutput)
	}
}
