package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	AppName    = "sota-headless"
	AppVersion = "1.0.0"

	DefaultUserAgent = "Sota Connect (v1.7.7/windows)"
)

var DefaultAPIBases = []string{
	"https://meowconnect.com/api/v1",
	"https://sota.ac/api/v1",
}

var (
	ErrInvalidSOTAAccessKey = errors.New("environment must contain real SOTA_ACCESS_KEY")
	ErrInvalidSOTAApiBases  = errors.New("SOTA_API_BASES must contain at least one API base URL")
)

type Config struct {
	BaseDir        string
	AccessKey      string
	APIEnabled     bool
	Listen         string
	HWID           string
	DeviceName     string
	UserAgent      string
	AcceptLanguage string
	LogLevel       string
	APIBases       []string
	CacheTTL       time.Duration
}

// StateDir returns the path to the state directory.
func (c *Config) StateDir() string {
	return filepath.Join(c.BaseDir, "state")
}

func Load(baseDir string) (Config, error) {
	if baseDir == "" {
		// Explicit env override — useful for OpenWrt/procd where cwd is /
		if envDir := env("SOTA_BASE_DIR", ""); envDir != "" {
			baseDir = envDir
		} else {
			wd, err := os.Getwd()
			if err != nil {
				return Config{}, fmt.Errorf("failed to get current working directory: %w", err)
			}
			baseDir = wd
		}
	}
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get absolute path for base directory: %w", err)
	}

	cacheTTL := parseDuration(env("SOTA_CACHE_TTL", "30m"), 30*time.Minute)

	cfg := Config{
		BaseDir:        baseDir,
		AccessKey:      env("SOTA_ACCESS_KEY", ""),
		APIEnabled:     parseBool(env("SOTA_API_ENABLED", "true")),
		Listen:         env("SOTA_LISTEN", env("SOTA_SERVER_LISTEN", "0.0.0.0:16698")),
		APIBases:       splitCSV(env("SOTA_API_BASES", strings.Join(DefaultAPIBases, ","))),
		HWID:           env("SOTA_HWID", ""),
		DeviceName:     env("SOTA_DEVICE_NAME", ""),
		UserAgent:      env("SOTA_USER_AGENT", DefaultUserAgent),
		AcceptLanguage: env("SOTA_ACCEPT_LANGUAGE", "ru"),
		LogLevel:       env("SOTA_LOG_LEVEL", "info"),
		CacheTTL:       cacheTTL,
	}
	if cfg.AccessKey == "" || cfg.AccessKey == "SOTA_ACCESS_KEY" {
		return Config{}, ErrInvalidSOTAAccessKey
	}
	if len(cfg.APIBases) == 0 {
		return Config{}, ErrInvalidSOTAApiBases
	}
	return cfg, nil
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on", "enable", "enabled":
		return true
	default:
		return false
	}
}

func splitCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(v)
	}
	return fallback
}
