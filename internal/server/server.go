package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"sota-headless/internal/config"
	"sota-headless/internal/controller"
	"sota-headless/internal/singbox"
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
	app := Server{Controller: c}.app()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			fmt.Printf("Error shutting down server: %v\n", err)
		}
	}()
	fmt.Printf("%s v%s listening on http://%s\n", config.AppName, config.AppVersion, addr)
	fmt.Println("Endpoints: GET /health /status /profile /locations /runtime-config /logs/tail ; POST /render /connect /disconnect /reload")
	if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s Server) app() *fiber.App {
	app := fiber.New(fiber.Config{
		ReadTimeout: 10 * time.Second,
		JSONEncoder: controller.MarshalJSON,
	})
	app.Use(recover.New())

	app.Get("/", s.health)
	app.Get("/health", s.health)
	app.Get("/status", s.status)
	app.Get("/profile", s.profile)
	app.Get("/locations", s.locations)
	app.Get("/device", s.device)
	app.Get("/runtime-config", s.runtimeConfig)
	app.Get("/logs/tail", s.logsTail)

	app.Post("/render", s.render)
	app.Post("/connect", s.connect)
	app.Post("/start", s.connect)
	app.Post("/disconnect", s.disconnect)
	app.Post("/stop", s.disconnect)
	app.Post("/reload", s.reload)

	app.Use(s.notFound)
	return app
}

func (s Server) health(c fiber.Ctx) error {
	return writeJSON(c, fiber.StatusOK, map[string]any{"ok": true, "service": config.AppName, "version": config.AppVersion, "status": s.Controller.Status()})
}

func (s Server) status(c fiber.Ctx) error {
	return writeJSON(c, fiber.StatusOK, s.Controller.Status())
}

func (s Server) profile(c fiber.Ctx) error {
	result, err := s.Controller.Profile(requestContext(c))
	return writeResult(c, s.Controller, result, err)
}

func (s Server) locations(c fiber.Ctx) error {
	result, err := s.Controller.Locations(requestContext(c))
	return writeResult(c, s.Controller, result, err)
}

func (s Server) device(c fiber.Ctx) error {
	return writeJSON(c, fiber.StatusOK, controller.Redact(map[string]any{"x_hwid": s.Controller.Device.HWID, "x_device_name": s.Controller.Device.DeviceName}))
}

func (s Server) runtimeConfig(c fiber.Ctx) error {
	result, err := s.Controller.RuntimeConfig(c.Query("raw") == "1")
	return writeResult(c, s.Controller, result, err)
}

func (s Server) logsTail(c fiber.Ctx) error {
	maxChars := int64(12000)
	if raw := c.Query("chars"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			maxChars = parsed
		}
	}
	return writeJSON(c, fiber.StatusOK, map[string]any{"path": s.Controller.SingBoxLog, "tail": singbox.TailFile(s.Controller.SingBoxLog, maxChars)})
}

func (s Server) render(c fiber.Ctx) error {
	body, err := parseBody(c)
	if err != nil {
		return writeJSON(c, fiber.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	gate := gateFromBody(body)
	runtimePath, snippet, cfg, err := s.Controller.Render(requestContext(c), gate)
	if err != nil {
		s.Controller.LastError = err.Error()
		return writeJSON(c, fiber.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return writeJSON(c, fiber.StatusOK, map[string]any{
		"ok":             true,
		"gate_id":        s.Controller.CurrentGateID,
		"runtime_config": runtimePath,
		"snippet":        controller.Redact(snippet),
		"config":         controller.Redact(cfg),
	})
}

func (s Server) connect(c fiber.Ctx) error {
	body, err := parseBody(c)
	if err != nil {
		return writeJSON(c, fiber.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	result, err := s.Controller.Start(requestContext(c), gateFromBody(body))
	return writeResult(c, s.Controller, result, err)
}

func (s Server) disconnect(c fiber.Ctx) error {
	return writeJSON(c, fiber.StatusOK, s.Controller.Stop())
}

func (s Server) reload(c fiber.Ctx) error {
	result, err := s.Controller.Reload(requestContext(c))
	return writeResult(c, s.Controller, result, err)
}

func (s Server) notFound(c fiber.Ctx) error {
	if knownPath(c.Path()) {
		return writeJSON(c, fiber.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
	return writeJSON(c, fiber.StatusNotFound, map[string]any{"error": "not found"})
}

func gateFromBody(body map[string]any) any {
	for _, key := range []string{"gate_id", "gate", "location"} {
		if value, ok := body[key]; ok {
			return value
		}
	}
	return nil
}

func parseBody(c fiber.Ctx) (map[string]any, error) {
	if len(c.Body()) == 0 {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err := c.Bind().Body(&body); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}
	if body == nil {
		body = map[string]any{}
	}
	return body, nil
}

func requestContext(c fiber.Ctx) context.Context {
	if ctx := c.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func writeResult(ctx fiber.Ctx, c *controller.Controller, value any, err error) error {
	if err != nil {
		c.LastError = err.Error()
		return writeJSON(ctx, fiber.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return writeJSON(ctx, fiber.StatusOK, value)
}

func writeJSON(c fiber.Ctx, status int, value any) error {
	data, err := controller.MarshalJSON(value)
	if err != nil {
		data, err = json.Marshal(map[string]any{"error": err.Error()})
		if err != nil {
			if err := c.Status(fiber.StatusInternalServerError).SendString(`{"error":"failed to marshal response"}`); err != nil {
				return fmt.Errorf("failed to write error response: %w", err)
			}
		}
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	if err := c.Status(status).Send(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	return nil
}

func knownPath(path string) bool {
	switch strings.TrimRight(path, "/") {
	case "", "/", "/health", "/status", "/profile", "/locations", "/device", "/runtime-config", "/logs/tail", "/render", "/connect", "/start", "/disconnect", "/stop", "/reload":
		return true
	default:
		return false
	}
}

func parseListen(listen string) (string, string, error) {
	if listen == "" {
		listen = "16698"
	}
	if !strings.Contains(listen, ":") {
		return "127.0.0.1", listen, nil
	}

	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", "", fmt.Errorf("invalid listen address: %w", err)
	}

	return host, port, nil
}
