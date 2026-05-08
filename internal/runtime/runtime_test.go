package runtimecfg_test

import (
	"context"
	"testing"

	"sota-headless/internal/config"
	runtimecfg "sota-headless/internal/runtime"
)

func TestBuildRemovesDNSOutboundAndCreatesTUN(t *testing.T) {
	t.Parallel()

	cfg, err := runtimecfg.Build(context.Background(), snippet(), runtimecfg.Options{Mode: config.ModeTUN, LogLevel: "info", RuleSetsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	inbounds := cfg["inbounds"].([]any)
	first := inbounds[0].(map[string]any)
	if first["type"] != "tun" {
		t.Fatalf("inbound type = %v", first["type"])
	}
	assertNoLegacyInboundFields(t, first)
	for _, item := range cfg["outbounds"].([]any) {
		if item.(map[string]any)["type"] == "dns" {
			t.Fatal("dns outbound was not removed")
		}
	}
}

func TestBuildProxyModeCreatesMixedInbound(t *testing.T) {
	t.Parallel()

	cfg, err := runtimecfg.Build(context.Background(), snippet(), runtimecfg.Options{Mode: config.ModeProxy, LogLevel: "info", ProxyListen: "127.0.0.1:2081", RuleSetsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	first := cfg["inbounds"].([]any)[0].(map[string]any)
	if first["type"] != "mixed" {
		t.Fatalf("inbound type = %v", first["type"])
	}
	if _, ok := first["sniff"]; ok {
		t.Fatal("mixed inbound must not use legacy sniff field")
	}
	assertNoLegacyInboundFields(t, first)
	if first["listen_port"] != 2081 {
		t.Fatalf("listen port = %v", first["listen_port"])
	}
	route := cfg["route"].(map[string]any)
	if _, ok := route["auto_detect_interface"]; ok {
		t.Fatal("proxy mode should not enable route auto_detect_interface")
	}
}

func assertNoLegacyInboundFields(t *testing.T, inbound map[string]any) {
	t.Helper()
	for _, field := range []string{"sniff", "sniff_timeout", "domain_strategy"} {
		if _, ok := inbound[field]; ok {
			t.Fatalf("inbound contains legacy field %q", field)
		}
	}
}

func snippet() map[string]any {
	return map[string]any{
		"configuration": map[string]any{
			"outbounds": []any{
				map[string]any{"type": "vless", "tag": "proxy", "server": "203.0.113.10"},
				map[string]any{"type": "dns", "tag": "dns-out"},
			},
			"route": map[string]any{
				"rule_set": []any{
					map[string]any{"type": "remote", "tag": "geoip", "url": "https://example.invalid/geoip.srs"},
				},
			},
		},
	}
}
