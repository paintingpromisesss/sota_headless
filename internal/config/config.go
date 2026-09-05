package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	AppName          = "sota-headless"
	DefaultUserAgent = "Sota Connect (v1.7.7/windows)"
)

var AppVersion = "1.1.0"

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
	LogFormat      string
	APIBases       []string
	CacheTTL       time.Duration
}

// StateDir returns the path to the state directory.
func (c *Config) StateDir() string {
	return filepath.Join(c.BaseDir, "state")
}

func defaultBaseDir() (string, error) {
	if envDir := env("SOTA_BASE_DIR", ""); envDir != "" {
		return envDir, nil
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if _, statErr := os.Stat(filepath.Join(exeDir, "sota-headless.env")); statErr == nil {
			return exeDir, nil
		}
		if runtime.GOOS == "windows" {
			return exeDir, nil
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return wd, nil
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

func Load(baseDir string) (Config, error) {
	explicitBaseDir := baseDir != ""
	if baseDir == "" {
		var err error
		baseDir, err = defaultBaseDir()
		if err != nil {
			return Config{}, err
		}
	}
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get absolute path for base directory: %w", err)
	}

	_ = loadEnvFile(filepath.Join(baseDir, "sota-headless.env"))
	_ = loadEnvFile(filepath.Join(baseDir, ".env"))

	if !explicitBaseDir {
		if envDir := env("SOTA_BASE_DIR", ""); envDir != "" {
			if absDir, absErr := filepath.Abs(envDir); absErr == nil {
				baseDir = absDir
			}
		}
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
		LogFormat:      env("SOTA_LOG_FORMAT", "text"),
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
