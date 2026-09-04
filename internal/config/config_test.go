package config_test

import (
	"os"
	"sota-headless/internal/config"
	"testing"
	"time"
)

func TestLoadFromEnvironment(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	unsetEnv(t, "SOTA_API_ENABLED")
	unsetEnv(t, "SOTA_API_BASES")
	unsetEnv(t, "SOTA_LOG_LEVEL")
	unsetEnv(t, "SOTA_LOG_FORMAT")
	t.Setenv("SOTA_ACCESS_KEY", "abc")
	t.Setenv("SOTA_API_ENABLED", "yes")
	t.Setenv("SOTA_API_BASES", "https://a/api/v1, https://b/api/v1")
	t.Setenv("SOTA_CACHE_TTL", "10m")
	t.Setenv("SOTA_LOG_LEVEL", "debug")
	t.Setenv("SOTA_LOG_FORMAT", "json")
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
	if len(cfg.APIBases) != 2 {
		t.Fatalf("api bases = %#v", cfg.APIBases)
	}
	if cfg.CacheTTL != 10*time.Minute {
		t.Fatalf("cache TTL = %v", cfg.CacheTTL)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("log level = %q, want debug", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("log format = %q, want json", cfg.LogFormat)
	}
}

func TestLoadRequiresAccessKey(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	_, err := config.Load(t.TempDir())
	if err == nil {
		t.Fatal("expected missing access key error")
	}
}

func TestAPIEnabledByDefault(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	unsetEnv(t, "SOTA_API_ENABLED")
	t.Setenv("SOTA_ACCESS_KEY", "abc")
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.APIEnabled {
		t.Fatal("expected APIEnabled=true by default")
	}
}

func TestLoadListenPrefersSOTAListen(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	unsetEnv(t, "SOTA_LISTEN")
	unsetEnv(t, "SOTA_SERVER_LISTEN")
	t.Setenv("SOTA_ACCESS_KEY", "abc")
	t.Setenv("SOTA_SERVER_LISTEN", "127.0.0.1:1111")
	t.Setenv("SOTA_LISTEN", "0.0.0.0:2222")

	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "0.0.0.0:2222" {
		t.Fatalf("listen = %q", cfg.Listen)
	}
}

func TestLoadListenSupportsLegacySOTAServerListen(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	unsetEnv(t, "SOTA_LISTEN")
	unsetEnv(t, "SOTA_SERVER_LISTEN")
	t.Setenv("SOTA_ACCESS_KEY", "abc")
	t.Setenv("SOTA_SERVER_LISTEN", "127.0.0.1:1111")

	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:1111" {
		t.Fatalf("listen = %q", cfg.Listen)
	}
}

func TestStateDirIsSubdirOfBaseDir(t *testing.T) {
	unsetEnv(t, "SOTA_ACCESS_KEY")
	t.Setenv("SOTA_ACCESS_KEY", "abc")
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StateDir() == "" {
		t.Fatal("StateDir() is empty")
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
