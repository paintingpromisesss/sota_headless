package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"sota-headless/internal/config"
	"sota-headless/internal/controller"
	"sota-headless/internal/provider"
)

type Server struct {
	Controller *controller.Controller
}

func ListenAndServe(ctx context.Context, c *controller.Controller, listen string) error {
	host, port, err := parseListen(listen)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(host, port)
	srv := Server{Controller: c}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		slog.Info("stopping HTTP server...")
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("error shutting down HTTP server", "error", err)
		} else {
			slog.Info("HTTP server stopped gracefully")
		}
	}()

	slog.Info("HTTP server listening", "service", config.AppName, "version", config.AppVersion, "address", "http://"+addr)
	slog.Info("available endpoints",
		"info", "/health, /status, /profile, /locations, /device",
		"sub", "/sub, /sub/mihomo, /sub/vless, /sub/base64, /sub/singbox",
		"ops", "POST /sub/refresh",
	)

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Info endpoints
	mux.HandleFunc("GET /{$}", s.health)
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /status", s.status)
	mux.HandleFunc("GET /profile", s.profile)
	mux.HandleFunc("GET /locations", s.locations)
	mux.HandleFunc("GET /device", s.device)

	// Subscription endpoints
	mux.HandleFunc("GET /sub", s.subMihomo)
	mux.HandleFunc("GET /sub/mihomo", s.subMihomo)
	mux.HandleFunc("GET /sub/vless", s.subVless)
	mux.HandleFunc("GET /sub/base64", s.subBase64)
	mux.HandleFunc("GET /sub/singbox", s.subSingbox)

	// Operations
	mux.HandleFunc("POST /sub/refresh", s.subRefresh)

	// Fallback for unmatched routes
	mux.HandleFunc("/", s.notFound)

	return withLoggingAndRecovery(mux)
}

func (s Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": config.AppName,
		"version": config.AppVersion,
		"status":  s.Controller.Status(),
	})
}

func (s Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Controller.Status())
}

func (s Server) profile(w http.ResponseWriter, r *http.Request) {
	result, err := s.Controller.Profile(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s Server) locations(w http.ResponseWriter, r *http.Request) {
	result, err := s.Controller.Locations(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s Server) device(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, controller.Redact(map[string]any{
		"x_hwid":        s.Controller.Device.HWID,
		"x_device_name": s.Controller.Device.DeviceName,
	}))
}

func (s Server) subMihomo(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.Controller.Nodes(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	yaml := provider.ToMihomoYAML(nodes)
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Header().Set("Profile-Update-Interval", "24")
	w.Header().Set("Subscription-Userinfo", "")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(yaml)
}

func (s Server) subVless(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.Controller.Nodes(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	lines := provider.ToVlessLines(nodes)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(lines))
}

func (s Server) subBase64(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.Controller.Nodes(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	b64 := provider.ToBase64(nodes)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b64))
}

func (s Server) subSingbox(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.Controller.Nodes(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	data, err := provider.ToSingboxJSON(nodes)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s Server) subRefresh(w http.ResponseWriter, r *http.Request) {
	slog.Info("manual cache refresh requested", "ip", clientIP(r))
	s.Controller.InvalidateCache()
	nodes, err := s.Controller.Nodes(r.Context())
	if err != nil {
		slog.Error("manual cache refresh failed", "error", err)
		writeError(w, r, err)
		return
	}
	slog.Info("manual cache refresh completed", "nodes_count", len(nodes))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"nodes_count": len(nodes),
		"refreshed":   true,
	})
}

func (s Server) notFound(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("endpoint returned error", "path", r.URL.Path, "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := controller.MarshalJSON(value)
	if err != nil {
		data, err = json.Marshal(map[string]any{"error": err.Error()})
		if err != nil {
			http.Error(w, `{"error":"failed to marshal response"}`, http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

func parseListen(listen string) (string, string, error) {
	if listen == "" {
		listen = "16698"
	}
	if !strings.Contains(listen, ":") {
		return "0.0.0.0", listen, nil
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", "", fmt.Errorf("invalid listen address: %w", err)
	}
	return host, port, nil
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *statusResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

func withLoggingAndRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		defer func() {
			if rec := recover(); rec != nil {
				rw.statusCode = http.StatusInternalServerError
				http.Error(rw, `{"error":"internal server error"}`, http.StatusInternalServerError)
				slog.Error("panic recovered in http handler", "error", fmt.Sprint(rec))
			}
			latency := time.Since(start)
			ip := clientIP(r)
			attrs := []any{
				"status", rw.statusCode,
				"method", r.Method,
				"path", r.URL.Path,
				"ip", ip,
				"latency", latency.Round(time.Microsecond).String(),
			}

			if rw.statusCode >= 500 {
				slog.Error("http request", attrs...)
			} else if rw.statusCode >= 400 {
				slog.Warn("http request", attrs...)
			} else {
				slog.Info("http request", attrs...)
			}
		}()

		next.ServeHTTP(rw, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
