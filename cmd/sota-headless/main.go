package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"sota-headless/internal/config"
	"sota-headless/internal/controller"
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
		listen  string
		profile bool
		locs    bool
		nodes   bool
		vless   bool
		base64  bool
		mihomo  bool
	)

	flag.StringVar(&listen, "listen", "", "HTTP listen address (default SOTA_LISTEN or 0.0.0.0:16698)")
	flag.BoolVar(&profile, "profile", false, "Fetch subscription profile and exit")
	flag.BoolVar(&locs, "locations", false, "Fetch server locations list and exit")
	flag.BoolVar(&nodes, "nodes", false, "Fetch all nodes with connection params and exit (JSON)")
	flag.BoolVar(&vless, "vless", false, "Print vless:// links for all nodes and exit")
	flag.BoolVar(&base64, "base64", false, "Print base64-encoded subscription and exit")
	flag.BoolVar(&mihomo, "mihomo", false, "Print Mihomo YAML proxy-provider and exit")
	flag.Parse()

	cfg, err := config.Load("")
	if err != nil {
		return fail(err)
	}
	if listen != "" {
		cfg.Listen = listen
	}

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
	_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	return 1
}
