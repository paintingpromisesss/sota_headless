package config_test

import (
	"os"
	"sota-headless/internal/config"
	"testing"
)

func TestLoadFromEnvironment(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	unsetEnv(t, "SOTA_API_ENABLED")
	unsetEnv(t, "SOTA_MODE")
	unsetEnv(t, "SOTA_API_BASES")
	t.Setenv("SOTA_ACCESS_KEY", "abc")
	t.Setenv("SOTA_API_ENABLED", "yes")
	t.Setenv("SOTA_MODE", "Proxy")
	t.Setenv("SOTA_API_BASES", "https://a/api/v1, https://b/api/v1")
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccessKey != "abc" {
		t.Fatalf("access key = %q", cfg.AccessKey)
	}
	if !cfg.APIEnabled {
		t.Fatal("expected APIEnabled=true")
	}
	if cfg.Mode != config.ModeProxy {
		t.Fatalf("mode = %q", cfg.Mode)
	}
	if len(cfg.APIBases) != 2 {
		t.Fatalf("api bases = %#v", cfg.APIBases)
	}
}

func TestLoadRequiresAccessKey(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	_, err := config.Load(t.TempDir())
	if err == nil {
		t.Fatal("expected missing access key error")
	}
}

func TestAPIDisabledByDefault(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	unsetEnv(t, "SOTA_API_ENABLED")
	t.Setenv("SOTA_ACCESS_KEY", "abc")
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIEnabled {
		t.Fatal("expected APIEnabled=false by default")
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}
