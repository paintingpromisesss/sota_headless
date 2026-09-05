package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"sota-headless/internal/config"
	"sota-headless/internal/controller"
	"sota-headless/internal/logger"
	"sota-headless/internal/provider"
	"sota-headless/internal/server"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Limit memory usage — important on routers with ≤256MB RAM
	debug.SetMemoryLimit(48 << 20) // 48 MiB

	var (
		listen    string
		baseDir   string
		logLevel  string
		logFormat string
		profile   bool
		locs      bool
		nodes     bool
		vless     bool
		base64    bool
		mihomo    bool
	)

	flag.StringVar(&baseDir, "base-dir", "", "Base directory for config and state (default SOTA_BASE_DIR or executable directory)")
	flag.StringVar(&listen, "listen", "", "HTTP listen address (default SOTA_LISTEN or 0.0.0.0:16698)")
	flag.StringVar(&logLevel, "log-level", "", "Log level: debug, info, warn, error (default SOTA_LOG_LEVEL or info)")
	flag.StringVar(&logFormat, "log-format", "", "Log format: text, json (default SOTA_LOG_FORMAT or text)")
	flag.BoolVar(&profile, "profile", false, "Fetch subscription profile and exit")
	flag.BoolVar(&locs, "locations", false, "Fetch server locations list and exit")
	flag.BoolVar(&nodes, "nodes", false, "Fetch all nodes with connection params and exit (JSON)")
	flag.BoolVar(&vless, "vless", false, "Print vless:// links for all nodes and exit")
	flag.BoolVar(&base64, "base64", false, "Print base64-encoded subscription and exit")
	flag.BoolVar(&mihomo, "mihomo", false, "Print Mihomo YAML proxy-provider and exit")
	flag.Parse()

	isCLICommand := profile || locs || nodes || vless || base64 || mihomo

	cfg, err := config.Load(baseDir)
	if err != nil {
		logger.Setup("info", "text", os.Stderr)
		return fail(err)
	}
	if listen != "" {
		cfg.Listen = listen
	}
	if logLevel != "" {
		cfg.LogLevel = logLevel
	} else if isCLICommand && os.Getenv("SOTA_LOG_LEVEL") == "" {
		cfg.LogLevel = "warn"
	}
	if logFormat != "" {
		cfg.LogFormat = logFormat
	}

	logger.Setup(cfg.LogLevel, cfg.LogFormat, os.Stderr)

	ctrl, err := controller.New(cfg)
	if err != nil {
		return fail(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch {
	case profile:
		result, err := ctrl.Profile(ctx)
		return printJSON(result, err)

	case locs:
		result, err := ctrl.Locations(ctx)
		return printJSON(result, err)

	case nodes:
		result, err := ctrl.Nodes(ctx)
		return printJSON(result, err)

	case vless:
		result, err := ctrl.Nodes(ctx)
		if err != nil {
			return fail(err)
		}
		fmt.Print(provider.ToVlessLines(result))
		return 0

	case base64:
		result, err := ctrl.Nodes(ctx)
		if err != nil {
			return fail(err)
		}
		fmt.Println(provider.ToBase64(result))
		return 0

	case mihomo:
		result, err := ctrl.Nodes(ctx)
		if err != nil {
			return fail(err)
		}
		_, _ = os.Stdout.Write(provider.ToMihomoYAML(result))
		return 0

	default:
		// When running as a Windows service under Service Control Manager
		if isSvc, svcErr := runWindowsService(func(svcCtx context.Context) error {
			return server.ListenAndServe(svcCtx, ctrl, cfg.Listen)
		}); isSvc {
			if svcErr != nil {
				return fail(svcErr)
			}
			return 0
		}

		// Default mode: run HTTP subscription server
		if err := server.ListenAndServe(ctx, ctrl, cfg.Listen); err != nil {
			return fail(err)
		}
		return 0
	}
}

func printJSON(value any, err error) int {
	if err != nil {
		return fail(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fail(err)
	}
	fmt.Println(string(data))
	return 0
}

func fail(err error) int {
	slog.Error("fatal error", "error", err)
	return 1
}
