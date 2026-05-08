package runtimecfg

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"sota-headless/internal/config"
	"sota-headless/internal/httpclient"
)

type Options struct {
	Mode          config.Mode
	RuleSetsDir   string
	UserAgent     string
	LogLevel      string
	ProxyListen   string
	CacheRuleSets bool
}

var (
	ErrConfigurationMissing          = fmt.Errorf("configuration not found in response")
	ErrInvalidConfigurationOutbounds = fmt.Errorf("configuration.outbounds outbounds in configuration")
	ErrProxyOutboundServerMissing    = fmt.Errorf("proxy outbound has no server field")
	ErrProxyOutboundMissing          = fmt.Errorf("no proxy outbound in configuration")
)

func Build(ctx context.Context, snippet map[string]any, opts Options) (map[string]any, error) {
	rawCfg, ok := snippet["configuration"].(map[string]any)
	if !ok {
		return nil, ErrConfigurationMissing
	}
	outbounds, ok := rawCfg["outbounds"].([]any)
	if !ok {
		return nil, ErrInvalidConfigurationOutbounds
	}
	proxy, err := findProxyOutbound(outbounds)
	if err != nil {
		return nil, err
	}

	//nolint:errcheck // no matter if proxy has no tag, we catch it later
	proxyTag, _ := proxy["tag"].(string)

	if proxyTag == "" {
		proxyTag = "proxy"
	}

	//nolint:errcheck // no matter if proxy has no server, we catch it later
	proxyServer, _ := proxy["server"].(string)
	if proxyServer == "" {
		return nil, ErrProxyOutboundServerMissing
	}

	outputOutbounds := make([]any, 0, len(outbounds)+1)
	hasDirect := false
	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if outbound["type"] == "dns" {
			continue
		}
		if outbound["tag"] == "direct" {
			hasDirect = true
		}
		outputOutbounds = append(outputOutbounds, outbound)
	}
	if !hasDirect {
		outputOutbounds = append(outputOutbounds, map[string]any{"tag": "direct", "type": "direct"})
	}

	//nolint:errcheck // no matter if route has no rule_set, we catch it later
	providerRoute, _ := rawCfg["route"].(map[string]any)
	ruleSets := maybeCacheRuleSets(ctx, providerRoute, opts)
	rules := []any{
		map[string]any{"action": "sniff"},
		map[string]any{
			"action": "hijack-dns",
			"mode":   "or",
			"rules": []any{
				map[string]any{"protocol": "dns"},
				map[string]any{"port": float64(53)},
			},
			"type": "logical",
		},
		map[string]any{"ip_is_private": true, "outbound": "direct"},
		proxyServerDirectRule(proxyServer),
		map[string]any{"domain": config.DefaultIPCheckDomain, "outbound": proxyTag},
	}
	for _, item := range ruleSets {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tag, ok := entry["tag"].(string); ok && tag != "" {
			rules = append(rules, map[string]any{"outbound": "direct", "rule_set": tag})
		}
	}

	route := map[string]any{
		"default_domain_resolver": map[string]any{"server": "dns_proxy", "strategy": "ipv4_only"},
		"final":                   proxyTag,
		"rule_set":                ruleSets,
		"rules":                   rules,
	}
	if opts.Mode == config.ModeTUN {
		route["auto_detect_interface"] = true
	}

	runtime := map[string]any{
		"dns": map[string]any{
			"client_subnet":     "0.0.0.0/0",
			"final":             "dns_proxy",
			"independent_cache": true,
			"rules":             []any{},
			"servers": []any{
				map[string]any{"detour": proxyTag, "server": config.DefaultDNSProxy, "tag": "dns_proxy", "type": "tcp"},
				map[string]any{"server": config.DefaultDNSDirect, "tag": "dns_direct", "type": "udp"},
			},
			"strategy": "ipv4_only",
		},
		"endpoints": rawCfg["endpoints"],
		"inbounds":  buildInbounds(opts),
		"log":       map[string]any{"level": opts.LogLevel, "timestamp": true},
		"outbounds": outputOutbounds,
		"route":     route,
	}
	if runtime["endpoints"] == nil {
		runtime["endpoints"] = []any{}
	}
	return StripRemovedDNSOutbounds(runtime), nil
}

func buildInbounds(opts Options) []any {
	if opts.Mode == config.ModeProxy {
		listenHost, listenPort := splitHostPort(opts.ProxyListen, "127.0.0.1", 2080)
		return []any{
			map[string]any{
				"listen":      listenHost,
				"listen_port": listenPort,
				"tag":         "mixed-in",
				"type":        "mixed",
			},
		}
	}
	return []any{
		map[string]any{
			"address":      []any{"10.0.0.1/30"},
			"auto_route":   true,
			"mtu":          float64(1500),
			"stack":        "mixed",
			"strict_route": true,
			"tag":          "tun-in",
			"type":         "tun",
		},
	}
}

func splitHostPort(raw, fallbackHost string, fallbackPort int) (string, int) {
	host, portRaw, err := net.SplitHostPort(raw)
	if err != nil {
		return fallbackHost, fallbackPort
	}
	port := fallbackPort
	if p, err := net.LookupPort("tcp", portRaw); err == nil {
		port = p
	}
	if host == "" {
		host = fallbackHost
	}
	return host, port
}

func findProxyOutbound(outbounds []any) (map[string]any, error) {
	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if ok && outbound["tag"] == "proxy" {
			return outbound, nil
		}
	}
	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		//nolint:errcheck // no matter if outbound has no type, we catch it later
		outboundType, _ := outbound["type"].(string)
		if outboundType != "direct" && outboundType != "block" && outboundType != "dns" {
			return outbound, nil
		}
	}
	return nil, ErrProxyOutboundMissing
}

func proxyServerDirectRule(server string) map[string]any {
	if ip := net.ParseIP(strings.TrimSpace(server)); ip != nil {
		suffix := 128
		if ip.To4() != nil {
			suffix = 32
		}
		return map[string]any{"ip_cidr": []any{fmt.Sprintf("%s/%d", ip.String(), suffix)}, "outbound": "direct"}
	}
	return map[string]any{"domain": strings.TrimSpace(server), "outbound": "direct"}
}

func maybeCacheRuleSets(ctx context.Context, route map[string]any, opts Options) []any {
	if route == nil {
		return []any{}
	}
	entries, ok := route["rule_set"].([]any)
	if !ok {
		return []any{}
	}
	if !opts.CacheRuleSets {
		return entries
	}
	if err := os.MkdirAll(opts.RuleSetsDir, 0700); err != nil {
		return entries
	}
	client := httpclient.New()
	cached := make([]any, 0, len(entries))
	for _, item := range entries {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		//nolint:errcheck // no matter if rule set has no tag, we catch it later
		tag, _ := entry["tag"].(string)
		if tag == "" {
			tag = "rule_set"
		}
		//nolint:errcheck // no matter if rule set has no url, we catch it later
		url, _ := entry["url"].(string)
		//nolint:errcheck // no matter if rule set has no type, we catch it later
		entryType, _ := entry["type"].(string)
		if entryType == "remote" && url != "" {
			dst := filepath.Join(opts.RuleSetsDir, tag+".srs")
			if err := downloadRuleSet(ctx, client, url, dst, opts.UserAgent); err == nil {
				abs, err := filepath.Abs(dst)
				if err != nil {
					continue
				}
				cached = append(cached, map[string]any{"type": "local", "tag": tag, "format": "binary", "path": abs})
				continue
			}
		}
		cached = append(cached, entry)
	}
	return cached
}

func downloadRuleSet(ctx context.Context, client *httpclient.Client, source, dst, userAgent string) error {
	if err := client.DownloadFile(ctx, source, map[string]string{"User-Agent": userAgent}, dst); err != nil {
		return fmt.Errorf("failed to download rule set from %s: %w", source, err)
	}
	return nil
}

func StripRemovedDNSOutbounds(runtime map[string]any) map[string]any {
	outbounds, ok := runtime["outbounds"].([]any)
	if !ok {
		return runtime
	}
	filtered := make([]any, 0, len(outbounds))
	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if ok && outbound["type"] == "dns" {
			continue
		}
		filtered = append(filtered, item)
	}
	runtime["outbounds"] = filtered
	return runtime
}

func WriteJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON for %s: %w", path, err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to move temp file to %s: %w", path, err)
	}
	return nil
}

func ReadJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON from %s: %w", path, err)
	}
	return out, nil
}
