package httpclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPSOnlyRejectsHTTP(t *testing.T) {
	client := New()
	err := client.GetJSON(context.Background(), "http://example.test/data.json", nil, &map[string]any{})
	if err == nil {
		t.Fatal("expected non-HTTPS URL to be rejected")
	}
}

func TestGetJSONAllowsHTTPWhenExplicitlyDisabled(t *testing.T) {
	client := New(
		WithHTTPSOnly(false),
		WithRoundTripper(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		})),
	)
	var out map[string]any
	if err := client.GetJSON(context.Background(), "http://example.test/data.json", nil, &out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Fatalf("unexpected response: %#v", out)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRequiresContext(t *testing.T) {
	client := New(WithHTTPSOnly(false))
	_, err := client.Do(nil, http.MethodGet, "http://example.test", nil, nil)
	if err == nil {
		t.Fatal("expected nil context error")
	}
}
