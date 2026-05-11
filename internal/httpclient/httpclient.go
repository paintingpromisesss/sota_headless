package httpclient

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrRedirectRefused     = fmt.Errorf("redirect refused from https to non-https URL")
	ErrNilContext          = fmt.Errorf("nil context")
	ErrAbsoluteURLRequired = fmt.Errorf("absolute URL required")
	ErrNonHTTPSURL         = errors.New("non-HTTPS URL")
	ErrUserInfoNotAllowed  = fmt.Errorf("URL with userinfo is not allowed")
	ErrResponseTooLarge    = fmt.Errorf("response exceeds maximum allowed size")
	ErrHTTPError           = fmt.Errorf("HTTP error response")
)

const (
	DefaultTimeout      = 20 * time.Second
	DefaultDownloadTime = 2 * time.Minute
	DefaultMaxBodyBytes = 10 << 20
)

type Client struct {
	httpClient  *http.Client
	maxBodySize int64
	httpsOnly   bool
}

type Option func(*Client)

func New(opts ...Option) *Client {
	c := &Client{
		maxBodySize: DefaultMaxBodyBytes,
		httpsOnly:   true,
	}
	c.httpClient = &http.Client{
		Timeout:   DefaultTimeout,
		Transport: secureTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: %s", ErrRedirectRefused, req.URL.Scheme)
			}
			return nil
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.httpClient.Timeout = timeout
		}
	}
}

func WithMaxBodySize(size int64) Option {
	return func(c *Client) {
		if size > 0 {
			c.maxBodySize = size
		}
	}
}

func WithHTTPSOnly(enabled bool) Option {
	return func(c *Client) {
		c.httpsOnly = enabled
	}
}

func WithRoundTripper(rt http.RoundTripper) Option {
	return func(c *Client) {
		if rt != nil {
			c.httpClient.Transport = rt
		}
	}
}

func (c *Client) GetJSON(ctx context.Context, endpoint string, headers map[string]string, target any) error {
	body, err := c.DoBytes(ctx, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("invalid JSON from %s: %w", endpoint, err)
	}
	return nil
}

func (c *Client) DownloadFile(ctx context.Context, endpoint string, headers map[string]string, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", dst, err)
	}
	resp, err := c.Do(ctx, http.MethodGet, endpoint, headers, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tmp := dst + ".tmp"
	//nolint:gosec // path forms internally
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", dst, err)
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		if err = os.Remove(tmp); err != nil {
			return fmt.Errorf("failed to clean up temp file for %s: %w", dst, err)
		}
		return fmt.Errorf("failed to download %s: %w", endpoint, copyErr)
	}
	if closeErr != nil {
		if err = os.Remove(tmp); err != nil {
			return fmt.Errorf("failed to clean up temp file for %s: %w", dst, err)
		}
		return fmt.Errorf("failed to finalize download for %s: %w", dst, closeErr)
	}
	if err = os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("failed to move temp file to %s: %w", dst, err)
	}
	return nil
}

func (c *Client) DoBytes(ctx context.Context, method, endpoint string, headers map[string]string, body io.Reader) ([]byte, error) {
	resp, err := c.Do(ctx, method, endpoint, headers, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", endpoint, err)
	}
	if int64(len(data)) > c.maxBodySize {
		return nil, fmt.Errorf("%w: %s; %d bytes", ErrResponseTooLarge, endpoint, c.maxBodySize)
	}
	return data, nil
}

func (c *Client) Do(ctx context.Context, method, endpoint string, headers map[string]string, body io.Reader) (*http.Response, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if err := c.validateURL(endpoint); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", endpoint, err)
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error for %s: %w", endpoint, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 501))
	if err != nil {
		return nil, fmt.Errorf("failed to read error response from %s: %w", endpoint, err)
	}
	if len(data) > 500 {
		data = data[:500]
	}
	return nil, fmt.Errorf("%w: %d from %s: %s", ErrHTTPError, resp.StatusCode, endpoint, string(data))
}

func (c *Client) validateURL(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: %s", ErrAbsoluteURLRequired, endpoint)
	}
	if c.httpsOnly && parsed.Scheme != "https" {
		return fmt.Errorf("%w: %s", ErrNonHTTPSURL, endpoint)
	}
	if parsed.User != nil {
		return fmt.Errorf("%w : %s", ErrUserInfoNotAllowed, endpoint)
	}
	return nil
}

func secureTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
}
