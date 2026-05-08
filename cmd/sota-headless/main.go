package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sota-headless/internal/config"
	"sota-headless/internal/controller"
	"sota-headless/internal/server"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		listen    string
		gateID    string
		profile   bool
		locations bool
		render    bool
		connect   bool
		raw       bool
	)
	flag.StringVar(&listen, "listen", "", "HTTP listen address, default SOTA_LISTEN or 127.0.0.1:16698")
	flag.BoolVar(&profile, "profile", false, "Fetch profile and exit")
	flag.BoolVar(&locations, "locations", false, "Fetch locations and exit")
	flag.BoolVar(&render, "render", false, "Fetch selected config, render runtime JSON and exit")
	flag.BoolVar(&connect, "connect", false, "Render and start sing-box, then keep running")
	flag.StringVar(&gateID, "gate-id", "", "Gate id or shortname, default: SOTA_GATE_ID or BST/best")
	flag.BoolVar(&raw, "raw", false, "Print unredacted JSON for local debugging")
	flag.Parse()

	cfg, err := config.Load("")
	if err != nil {
		return fail(err)
	}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "listen":
			cfg.Listen = listen
		case "gate-id":
			cfg.GateID = gateID
		}
	})
	ctrl, err := controller.New(cfg)
	if err != nil {
		return fail(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer ctrl.Stop()

	switch {
	case profile:
		result, err := ctrl.Profile(ctx)
		return printResult(result, err, true)
	case locations:
		result, err := ctrl.Locations(ctx)
		return printResult(result, err, true)
	case render:
		path, snippet, runtime, err := ctrl.Render(ctx, cfg.GateID)
		result := map[string]any{"runtime_config": path, "gate_id": ctrl.CurrentGateID, "snippet": snippet, "config": runtime}
		return printResult(result, err, raw)
	case connect:
		result, err := ctrl.Start(ctx, cfg.GateID)
		if code := printResult(result, err, true); code != 0 {
			return code
		}
		<-ctx.Done()
		return 0
	case cfg.APIEnabled:
		if err := server.ListenAndServe(ctx, ctrl, cfg.Listen); err != nil {
			return fail(err)
		}
		time.Sleep(100 * time.Millisecond)
		return 0
	default:
		result, err := ctrl.Start(ctx, cfg.GateID)
		if code := printResult(result, err, true); code != 0 {
			return code
		}
		<-ctx.Done()
		return 0
	}
}

func printResult(value any, err error, unredacted bool) int {
	if err != nil {
		return fail(err)
	}
	if !unredacted {
		value = controller.Redact(value)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fail(err)
	}
	fmt.Println(string(data))
	return 0
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	return 1
}
