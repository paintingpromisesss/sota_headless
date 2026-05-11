package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	AppName    = "sota-headless"
	AppVersion = "1.0.0"

	DefaultUserAgent     = "Sota Connect (v1.7.7/windows)"
	DefaultDNSProxy      = "94.140.14.14"
	DefaultDNSDirect     = "94.140.15.15"
	DefaultIPCheckDomain = "ip.accessly.app"
)

var DefaultAPIBases = []string{
	"https://meowconnect.com/api/v1",
	"https://sota.ac/api/v1",
}

var (
	ErrInvalidSOTAAccessKey = errors.New("environment must contain real SOTA_ACCESS_KEY")
	ErrInvalidSOTAApiBases  = errors.New("SOTA_API_BASES must contain at least one API base URL")
)

type Mode string

const (
	ModeTUN   Mode = "TUN"
	ModeProxy Mode = "Proxy"
)

type Config struct {
	BaseDir        string
	AccessKey      string
	APIEnabled     bool
	Listen         string
	GateID         string
	HWID           string
	DeviceName     string
	UserAgent      string
	AcceptLanguage string
	LogLevel       string
	SingBoxBin     string
	SingBoxDir     string
	SingBoxVersion string
	ProxyListen    string
	Mode           Mode
	APIBases       []string
}

func Load(baseDir string) (Config, error) {
	if baseDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Config{}, fmt.Errorf("failed to get current working directory: %w", err)
		}
		baseDir = wd
	}
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get absolute path for base directory: %w", err)
	}

	cfg := Config{
		BaseDir:        baseDir,
		AccessKey:      env("SOTA_ACCESS_KEY", ""),
		APIEnabled:     parseBool(env("SOTA_API_ENABLED", "false")),
		Mode:           parseMode(env("SOTA_MODE", string(ModeTUN))),
		Listen:         env("SOTA_SERVER_LISTEN", "127.0.0.1:16698"),
		GateID:         env("SOTA_GATE_ID", ""),
		APIBases:       splitCSV(env("SOTA_API_BASES", strings.Join(DefaultAPIBases, ","))),
		HWID:           env("SOTA_HWID", ""),
		DeviceName:     env("SOTA_DEVICE_NAME", ""),
		UserAgent:      env("SOTA_USER_AGENT", DefaultUserAgent),
		AcceptLanguage: env("SOTA_ACCEPT_LANGUAGE", "ru"),
		LogLevel:       env("SOTA_LOG_LEVEL", "info"),
		SingBoxBin:     env("SING_BOX_BIN", ""),
		SingBoxDir:     env("SING_BOX_DIR", "./bin"),
		SingBoxVersion: env("SING_BOX_VERSION", "v1.13.11"),
		ProxyListen:    env("SOTA_PROXY_LISTEN", "127.0.0.1:2080"),
	}
	if cfg.AccessKey == "" || cfg.AccessKey == "SOTA_ACCESS_KEY" {
		return Config{}, ErrInvalidSOTAAccessKey
	}
	if len(cfg.APIBases) == 0 {
		return Config{}, ErrInvalidSOTAApiBases
	}
	if cfg.SingBoxDir != "" && !filepath.IsAbs(cfg.SingBoxDir) {
		cfg.SingBoxDir = filepath.Join(baseDir, cfg.SingBoxDir)
	}
	return cfg, nil
}

func parseMode(raw string) Mode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "proxy":
		return ModeProxy
	default:
		return ModeTUN
	}
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
