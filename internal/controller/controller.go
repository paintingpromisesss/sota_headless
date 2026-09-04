package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"sota-headless/internal/config"
	"sota-headless/internal/provider"
	"sota-headless/internal/sota"
)

type Controller struct {
	Client    *sota.Client
	Config    *config.Config
	Device    sota.Device
	StateDir  string
	LastError string

	mu        sync.RWMutex
	cachedAt  time.Time
	cache     []provider.Node
	cacheTTL  time.Duration
}

func New(cfg config.Config) (*Controller, error) {
	stateDir := cfg.StateDir()
	device, err := sota.LoadOrCreateDevice(stateDir, cfg.HWID, cfg.DeviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to load or create device: %w", err)
	}
	client := sota.NewClient(cfg.AccessKey, device, cfg.APIBases, cfg.UserAgent, cfg.AcceptLanguage)
	cfgCopy := cfg
	return &Controller{
		Config:   &cfgCopy,
		Client:   client,
		Device:   device,
		StateDir: stateDir,
		cacheTTL: cfg.CacheTTL,
	}, nil
}

func (c *Controller) Profile(ctx context.Context) (map[string]any, error) {
	slog.Debug("fetching subscription profile")
	profile, err := c.Client.Profile(ctx)
	if err != nil {
		slog.Error("failed to fetch subscription profile", "error", err)
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	slog.Debug("successfully fetched subscription profile")
	return profile, nil
}

func (c *Controller) Locations(ctx context.Context) ([]map[string]any, error) {
	slog.Debug("fetching locations list")
	locations, err := c.Client.Locations(ctx)
	if err != nil {
		slog.Error("failed to fetch locations list", "error", err)
		return nil, fmt.Errorf("failed to fetch locations: %w", err)
	}
	slog.Debug("successfully fetched locations list", "count", len(locations))
	return locations, nil
}

// Nodes returns all parsed nodes, using cache if still fresh.
func (c *Controller) Nodes(ctx context.Context) ([]provider.Node, error) {
	c.mu.RLock()
	if len(c.cache) > 0 && time.Since(c.cachedAt) < c.cacheTTL {
		nodes := c.cache
		age := time.Since(c.cachedAt).Round(time.Second)
		c.mu.RUnlock()
		slog.Debug("serving nodes from cache", "nodes_count", len(nodes), "cache_age", age.String(), "cache_ttl", c.cacheTTL.String())
		return nodes, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if len(c.cache) > 0 && time.Since(c.cachedAt) < c.cacheTTL {
		nodes := c.cache
		age := time.Since(c.cachedAt).Round(time.Second)
		slog.Debug("serving nodes from cache", "nodes_count", len(nodes), "cache_age", age.String(), "cache_ttl", c.cacheTTL.String())
		return nodes, nil
	}

	if len(c.cache) > 0 {
		slog.Info("node cache expired, refreshing from Sota API", "cached_age", time.Since(c.cachedAt).Round(time.Second).String(), "cache_ttl", c.cacheTTL.String())
	} else {
		slog.Info("node cache empty, fetching from Sota API")
	}

	start := time.Now()
	nodes, err := provider.FetchAllNodes(ctx, c.Client)
	if err != nil {
		c.LastError = err.Error()
		slog.Error("failed to fetch nodes from Sota API", "error", err, "duration", time.Since(start).Round(time.Millisecond).String())
		return nil, err
	}
	c.cache = nodes
	c.cachedAt = time.Now()
	c.LastError = ""
	slog.Info("node cache updated successfully", "nodes_count", len(nodes), "cache_ttl", c.cacheTTL.String(), "duration", time.Since(start).Round(time.Millisecond).String())
	return nodes, nil
}

// InvalidateCache forces a fresh fetch on next Nodes() call.
func (c *Controller) InvalidateCache() {
	c.mu.Lock()
	prevCount := len(c.cache)
	c.cache = nil
	c.cachedAt = time.Time{}
	c.mu.Unlock()
	slog.Info("node cache invalidated", "previous_nodes_count", prevCount)
}

func (c *Controller) Status() map[string]any {
	c.mu.RLock()
	cacheAge := ""
	cacheCount := 0
	if !c.cachedAt.IsZero() {
		cacheAge = time.Since(c.cachedAt).Round(time.Second).String()
		cacheCount = len(c.cache)
	}
	c.mu.RUnlock()

	return map[string]any{
		"version":     config.AppVersion,
		"cache_nodes": cacheCount,
		"cache_age":   cacheAge,
		"cache_ttl":   c.cacheTTL.String(),
		"api_base":    nullIfEmpty(c.Client.ActiveBase),
		"last_error":  c.LastError,
		"device": map[string]any{
			"x_device_name": c.Device.DeviceName,
			"x_hwid":        RedactValue(c.Device.HWID),
		},
	}
}

func SelectGateID(locations []map[string]any, requested any) (int, error) {
	token := strings.TrimSpace(fmt.Sprint(requested))
	if requested != nil && token != "" && token != "<nil>" {
		if id, err := strconv.Atoi(token); err == nil {
			return id, nil
		}
		for _, item := range locations {
			if strings.EqualFold(stringField(item, "shortname"), token) || strings.EqualFold(stringField(item, "name"), token) {
				return intField(item, "id")
			}
		}
		return 0, fmt.Errorf("requested gate/location not found: %v", requested)
	}
	for _, item := range locations {
		if strings.EqualFold(stringField(item, "shortname"), "BST") {
			return intField(item, "id")
		}
	}
	for _, item := range locations {
		name := strings.ToLower(stringField(item, "name"))
		if strings.Contains(name, "луч") || strings.Contains(name, "best") {
			return intField(item, "id")
		}
	}
	for _, item := range locations {
		if isBest, _ := item["is_best"].(bool); isBest {
			return intField(item, "id")
		}
	}
	if len(locations) == 0 {
		return 0, fmt.Errorf("no locations returned by connection/list")
	}
	return intField(locations[0], "id")
}

var sensitiveKeys = map[string]struct{}{
	"access_key":   {},
	"x-access-key": {},
	"X-Access-Key": {},
	"uuid":         {},
	"public_key":   {},
	"short_id":     {},
	"password":     {},
	"token":        {},
}

func RedactValue(value any) any {
	s := fmt.Sprint(value)
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func Redact(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if _, ok := sensitiveKeys[key]; ok {
				out[key] = RedactValue(item)
				continue
			}
			if _, ok := sensitiveKeys[strings.ToLower(key)]; ok {
				out[key] = RedactValue(item)
				continue
			}
			out[key] = Redact(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = Redact(item)
		}
		return out
	default:
		return value
	}
}

func MarshalJSON(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}

func stringField(item map[string]any, key string) string {
	if v, ok := item[key].(string); ok {
		return v
	}
	return fmt.Sprint(item[key])
}

func intField(item map[string]any, key string) (int, error) {
	switch v := item[key].(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		return int(i), err
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("location is missing numeric %s", key)
	}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
