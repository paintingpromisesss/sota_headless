package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"sota-headless/internal/sota"
)

type Node struct {
	GateID      int    `json:"gate_id"`
	Name        string `json:"name"`
	CountryCode string `json:"country_code"`
	Emoji       string `json:"emoji"`
	Server      string `json:"server"`
	Port        int    `json:"port"`
	UUID        string `json:"uuid"`
	Flow        string `json:"flow"`
	SNI         string `json:"sni"`
	PublicKey   string `json:"public_key"`
	ShortID     string `json:"short_id"`
	Fingerprint string `json:"fingerprint"`
}

func (n Node) DisplayName() string {
	var parts []string
	if n.Emoji != "" {
		parts = append(parts, n.Emoji)
	}
	if n.CountryCode != "" {
		parts = append(parts, n.CountryCode)
	}
	if n.Name != "" && !strings.EqualFold(n.Name, n.CountryCode) {
		parts = append(parts, n.Name)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Sota-%d", n.GateID)
	}
	return strings.Join(parts, " ")
}

// ToVlessURL generates standard vless:// link
func (n Node) ToVlessURL() string {
	q := url.Values{}
	q.Set("type", "tcp")
	q.Set("security", "reality")
	if n.Flow != "" {
		q.Set("flow", n.Flow)
	}
	if n.SNI != "" {
		q.Set("sni", n.SNI)
	}
	if n.Fingerprint != "" {
		q.Set("fp", n.Fingerprint)
	}
	if n.PublicKey != "" {
		q.Set("pbk", n.PublicKey)
	}
	if n.ShortID != "" {
		q.Set("sid", n.ShortID)
	}

	tag := url.QueryEscape(n.DisplayName())
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", n.UUID, n.Server, n.Port, q.Encode(), tag)
}

// ToMihomoProxyMap converts node to Mihomo / Clash.Meta proxy definition
func (n Node) ToMihomoProxyMap() map[string]any {
	m := map[string]any{
		"name":               n.DisplayName(),
		"type":               "vless",
		"server":             n.Server,
		"port":               n.Port,
		"uuid":               n.UUID,
		"network":            "tcp",
		"tls":                true,
		"udp":                true,
		"servername":         n.SNI,
		"client-fingerprint": n.Fingerprint,
	}
	if n.Flow != "" {
		m["flow"] = n.Flow
	}
	realityOpts := map[string]any{}
	if n.PublicKey != "" {
		realityOpts["public-key"] = n.PublicKey
	}
	if n.ShortID != "" {
		realityOpts["short-id"] = n.ShortID
	}
	if len(realityOpts) > 0 {
		m["reality-opts"] = realityOpts
	}
	return m
}

// ToSingboxOutbound converts node to sing-box outbound
func (n Node) ToSingboxOutbound() map[string]any {
	out := map[string]any{
		"type":        "vless",
		"tag":         n.DisplayName(),
		"server":      n.Server,
		"server_port": n.Port,
		"uuid":        n.UUID,
		"tls": map[string]any{
			"enabled":     true,
			"server_name": n.SNI,
			"utls": map[string]any{
				"enabled":     true,
				"fingerprint": n.Fingerprint,
			},
			"reality": map[string]any{
				"enabled":    true,
				"public_key": n.PublicKey,
				"short_id":   n.ShortID,
			},
		},
	}
	if n.Flow != "" {
		out["flow"] = n.Flow
	}
	return out
}

// FetchAllNodes queries Sota API, fetches all locations and their Reality configs concurrently
func FetchAllNodes(ctx context.Context, client *sota.Client) ([]Node, error) {
	locations, err := client.Locations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch locations: %w", err)
	}

	type locInfo struct {
		id          int
		name        string
		countryCode string
		emoji       string
	}

	locs := make([]locInfo, 0, len(locations))
	for _, l := range locations {
		id, err := extractInt(l, "id")
		if err != nil || id <= 0 {
			continue
		}
		locs = append(locs, locInfo{
			id:          id,
			name:        extractString(l, "name"),
			countryCode: extractString(l, "shortname"),
			emoji:       extractString(l, "emoji"),
		})
	}

	if len(locs) == 0 {
		return nil, fmt.Errorf("no valid locations found in API response")
	}

	// Concurrently query connection snippet for each location
	results := make([]Node, 0, len(locs))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to 4 to be polite with API
	sem := make(chan struct{}, 4)

	for _, loc := range locs {
		wg.Add(1)
		go func(l locInfo) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			snippet, err := client.Connect(ctx, l.id)
			if err != nil {
				return
			}

			node, err := parseSnippet(snippet, l.id, l.name, l.countryCode, l.emoji)
			if err != nil {
				return
			}

			mu.Lock()
			results = append(results, node)
			mu.Unlock()
		}(loc)
	}

	wg.Wait()

	if len(results) == 0 {
		return nil, fmt.Errorf("failed to parse any nodes from Sota connect API")
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].GateID < results[j].GateID
	})

	return results, nil
}

func parseSnippet(snippet map[string]any, gateID int, name, countryCode, emoji string) (Node, error) {
	rawCfg, ok := snippet["configuration"].(map[string]any)
	if !ok {
		return Node{}, fmt.Errorf("configuration missing")
	}
	outbounds, ok := rawCfg["outbounds"].([]any)
	if !ok {
		return Node{}, fmt.Errorf("outbounds missing")
	}

	for _, item := range outbounds {
		out, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if out["type"] != "vless" {
			continue
		}

		server := extractString(out, "server")
		port, _ := extractInt(out, "server_port")
		if port == 0 {
			port = 443
		}
		uuid := extractString(out, "uuid")
		flow := extractString(out, "flow")

		var sni, fp, pbk, sid string
		if tls, ok := out["tls"].(map[string]any); ok {
			sni = extractString(tls, "server_name")
			if utls, ok := tls["utls"].(map[string]any); ok {
				fp = extractString(utls, "fingerprint")
			}
			if reality, ok := tls["reality"].(map[string]any); ok {
				pbk = extractString(reality, "public_key")
				sid = extractString(reality, "short_id")
			}
		}

		if server == "" || uuid == "" || pbk == "" {
			continue
		}

		return Node{
			GateID:      gateID,
			Name:        name,
			CountryCode: countryCode,
			Emoji:       emoji,
			Server:      server,
			Port:        port,
			UUID:        uuid,
			Flow:        flow,
			SNI:         sni,
			PublicKey:   pbk,
			ShortID:     sid,
			Fingerprint: fp,
		}, nil
	}

	return Node{}, fmt.Errorf("no vless outbound found in snippet")
}

func ToVlessLines(nodes []Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(n.ToVlessURL())
		b.WriteByte('\n')
	}
	return b.String()
}

func ToBase64(nodes []Node) string {
	lines := ToVlessLines(nodes)
	return base64.StdEncoding.EncodeToString([]byte(lines))
}

func ToSingboxJSON(nodes []Node) ([]byte, error) {
	outbounds := make([]any, 0, len(nodes))
	for _, n := range nodes {
		outbounds = append(outbounds, n.ToSingboxOutbound())
	}
	return json.MarshalIndent(map[string]any{"outbounds": outbounds}, "", "  ")
}

// ToMihomoYAML generates clean Clash / Mihomo proxy-provider YAML without external dependencies
func ToMihomoYAML(nodes []Node) []byte {
	var b strings.Builder
	b.WriteString("# Sota Headless generated Mihomo/Clash Proxy Provider\n")
	b.WriteString("proxies:\n")
	for _, n := range nodes {
		b.WriteString("  - name: " + quote(n.DisplayName()) + "\n")
		b.WriteString("    type: vless\n")
		b.WriteString("    server: " + quote(n.Server) + "\n")
		b.WriteString(fmt.Sprintf("    port: %d\n", n.Port))
		b.WriteString("    uuid: " + quote(n.UUID) + "\n")
		b.WriteString("    network: tcp\n")
		b.WriteString("    tls: true\n")
		b.WriteString("    udp: true\n")
		if n.Flow != "" {
			b.WriteString("    flow: " + quote(n.Flow) + "\n")
		}
		if n.SNI != "" {
			b.WriteString("    servername: " + quote(n.SNI) + "\n")
		}
		if n.Fingerprint != "" {
			b.WriteString("    client-fingerprint: " + quote(n.Fingerprint) + "\n")
		}
		if n.PublicKey != "" || n.ShortID != "" {
			b.WriteString("    reality-opts:\n")
			if n.PublicKey != "" {
				b.WriteString("      public-key: " + quote(n.PublicKey) + "\n")
			}
			if n.ShortID != "" {
				b.WriteString("      short-id: " + quote(n.ShortID) + "\n")
			}
		}
	}
	return []byte(b.String())
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func extractString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func extractInt(m map[string]any, key string) (int, error) {
	v, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	switch val := v.(type) {
	case int:
		return val, nil
	case float64:
		return int(val), nil
	case int64:
		return int(val), nil
	case json.Number:
		i, err := val.Int64()
		return int(i), err
	default:
		return 0, fmt.Errorf("invalid int %v", v)
	}
}
