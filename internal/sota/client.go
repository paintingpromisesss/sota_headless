package sota

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"sota-headless/internal/httpclient"
)

type Client struct {
	AccessKey      string
	Device         Device
	Bases          []string
	UserAgent      string
	AcceptLanguage string
	ActiveBase     string
	HTTPClient     *httpclient.Client
}

func NewClient(accessKey string, device Device, bases []string, userAgent, acceptLanguage string) *Client {
	return &Client{
		AccessKey:      accessKey,
		Device:         device,
		Bases:          normalizeBases(bases),
		UserAgent:      userAgent,
		AcceptLanguage: acceptLanguage,
		HTTPClient:     httpclient.New(),
	}
}

func (c *Client) Profile(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.RequestJSON(ctx, "/public/subscription/profile", nil, "GET", &out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("profile response is not an object")
	}
	return out, nil
}

func (c *Client) Locations(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	if err := c.RequestJSON(ctx, "/public/connection/list", nil, "GET", &out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("connection/list response is not an array")
	}
	return out, nil
}

func (c *Client) Connect(ctx context.Context, gateID int) (map[string]any, error) {
	var out map[string]any
	if err := c.RequestJSON(ctx, "/public/connection/connect", url.Values{"gate_id": {fmt.Sprintf("%d", gateID)}}, "GET", &out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("connection/connect response is not an object")
	}
	return out, nil
}

func (c *Client) RequestJSON(ctx context.Context, path string, query url.Values, method string, target any) error {
	candidates := make([]string, 0, len(c.Bases)+1)
	if c.ActiveBase != "" {
		candidates = append(candidates, c.ActiveBase)
	}
	for _, base := range c.Bases {
		if base != "" && base != c.ActiveBase {
			candidates = append(candidates, base)
		}
	}
	var lastErr error
	for _, base := range candidates {
		if err := c.requestJSONBase(ctx, base, path, query, method, target); err != nil {
			lastErr = err
			continue
		}
		c.ActiveBase = base
		return nil
	}
	return fmt.Errorf("all API bases failed for %s: %w", path, lastErr)
}

func (c *Client) requestJSONBase(ctx context.Context, base, path string, query url.Values, method string, target any) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	endpoint := base + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	client := c.HTTPClient
	if client == nil {
		client = httpclient.New()
	}
	return client.GetJSON(ctx, endpoint, c.Headers(), target)
}

func (c *Client) Headers() map[string]string {
	return map[string]string{
		"User-Agent":      c.UserAgent,
		"X-Device-Name":   c.Device.DeviceName,
		"Accept-Language": c.AcceptLanguage,
		"Content-Type":    "application/json",
		"X-HWID":          c.Device.HWID,
		"X-Access-Key":    c.AccessKey,
	}
}

func normalizeBases(bases []string) []string {
	out := make([]string, 0, len(bases))
	for _, base := range bases {
		base = strings.TrimRight(strings.TrimSpace(base), "/")
		if base == "" {
			continue
		}
		if !strings.HasSuffix(base, "/api/v1") {
			base += "/api/v1"
		}
		out = append(out, base)
	}
	return out
}
