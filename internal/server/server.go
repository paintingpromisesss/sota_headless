package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"

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
	fmt.Println("Endpoints: GET /health /status /profile /locations /device")
	fmt.Println("           GET /sub/mihomo /sub/vless /sub/base64 /sub/singbox")
	fmt.Println("           POST /sub/refresh")
	if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s Server) app() *fiber.App {
	app := fiber.New(fiber.Config{
		ReadTimeout: 30 * time.Second,
		JSONEncoder: controller.MarshalJSON,
	})
	app.Use(recover.New())

	// Info endpoints
	app.Get("/", s.health)
	app.Get("/health", s.health)
	app.Get("/status", s.status)
	app.Get("/profile", s.profile)
	app.Get("/locations", s.locations)
	app.Get("/device", s.device)

	// Subscription endpoints
	app.Get("/sub", s.subMihomo)          // default: Mihomo YAML
	app.Get("/sub/mihomo", s.subMihomo)   // Mihomo / Clash.Meta proxy-provider YAML
	app.Get("/sub/vless", s.subVless)     // plain vless:// links (newline separated)
	app.Get("/sub/base64", s.subBase64)   // base64-encoded vless:// links (for generic clients)
	app.Get("/sub/singbox", s.subSingbox) // sing-box outbounds JSON

	// Force cache refresh
	app.Post("/sub/refresh", s.subRefresh)

	app.Use(s.notFound)
	return app
}

func (s Server) health(c fiber.Ctx) error {
	return writeJSON(c, fiber.StatusOK, map[string]any{
		"ok":      true,
		"service": config.AppName,
		"version": config.AppVersion,
		"status":  s.Controller.Status(),
	})
}

func (s Server) status(c fiber.Ctx) error {
	return writeJSON(c, fiber.StatusOK, s.Controller.Status())
}

func (s Server) profile(c fiber.Ctx) error {
	result, err := s.Controller.Profile(requestContext(c))
	if err != nil {
		return writeError(c, err)
	}
	return writeJSON(c, fiber.StatusOK, result)
}

func (s Server) locations(c fiber.Ctx) error {
	result, err := s.Controller.Locations(requestContext(c))
	if err != nil {
		return writeError(c, err)
	}
	return writeJSON(c, fiber.StatusOK, result)
}

func (s Server) device(c fiber.Ctx) error {
	return writeJSON(c, fiber.StatusOK, controller.Redact(map[string]any{
		"x_hwid":        s.Controller.Device.HWID,
		"x_device_name": s.Controller.Device.DeviceName,
	}))
}

func (s Server) subMihomo(c fiber.Ctx) error {
	nodes, err := s.Controller.Nodes(requestContext(c))
	if err != nil {
		return writeError(c, err)
	}
	yaml := provider.ToMihomoYAML(nodes)
	c.Set(fiber.HeaderContentType, "text/yaml; charset=utf-8")
	c.Set("Profile-Update-Interval", "24")
	c.Set("Subscription-Userinfo", "")
	return c.Status(fiber.StatusOK).Send(yaml)
}

func (s Server) subVless(c fiber.Ctx) error {
	nodes, err := s.Controller.Nodes(requestContext(c))
	if err != nil {
		return writeError(c, err)
	}
	lines := provider.ToVlessLines(nodes)
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	return c.Status(fiber.StatusOK).SendString(lines)
}

func (s Server) subBase64(c fiber.Ctx) error {
	nodes, err := s.Controller.Nodes(requestContext(c))
	if err != nil {
		return writeError(c, err)
	}
	b64 := provider.ToBase64(nodes)
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	return c.Status(fiber.StatusOK).SendString(b64)
}

func (s Server) subSingbox(c fiber.Ctx) error {
	nodes, err := s.Controller.Nodes(requestContext(c))
	if err != nil {
		return writeError(c, err)
	}
	data, err := provider.ToSingboxJSON(nodes)
	if err != nil {
		return writeError(c, err)
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Status(fiber.StatusOK).Send(data)
}

func (s Server) subRefresh(c fiber.Ctx) error {
	s.Controller.InvalidateCache()
	nodes, err := s.Controller.Nodes(requestContext(c))
	if err != nil {
		return writeError(c, err)
	}
	return writeJSON(c, fiber.StatusOK, map[string]any{
		"ok":          true,
		"nodes_count": len(nodes),
		"refreshed":   true,
	})
}

func (s Server) notFound(c fiber.Ctx) error {
	known := []string{
		"", "/", "/health", "/status", "/profile", "/locations", "/device",
		"/sub", "/sub/mihomo", "/sub/vless", "/sub/base64", "/sub/singbox", "/sub/refresh",
	}
	path := strings.TrimRight(c.Path(), "/")
	for _, k := range known {
		if path == k {
			return writeJSON(c, fiber.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		}
	}
	return writeJSON(c, fiber.StatusNotFound, map[string]any{"error": "not found"})
}

func requestContext(c fiber.Ctx) context.Context {
	if ctx := c.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func writeError(c fiber.Ctx, err error) error {
	return writeJSON(c, fiber.StatusInternalServerError, map[string]any{"error": err.Error()})
}

func writeJSON(c fiber.Ctx, status int, value any) error {
	data, err := controller.MarshalJSON(value)
	if err != nil {
		data, err = json.Marshal(map[string]any{"error": err.Error()})
		if err != nil {
			_ = c.Status(fiber.StatusInternalServerError).SendString(`{"error":"failed to marshal response"}`)
			return nil
		}
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Status(status).Send(append(data, '\n'))
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
