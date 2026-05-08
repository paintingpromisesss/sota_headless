package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"sota-headless/internal/config"
	runtimecfg "sota-headless/internal/runtime"
	"sota-headless/internal/singbox"
	"sota-headless/internal/sota"
)

type Controller struct {
	Config        config.Config
	Client        *sota.Client
	Device        sota.Device
	StateDir      string
	RuntimeDir    string
	RuleSetsDir   string
	RuntimePath   string
	SingBoxLog    string
	LastError     string
	LastSnippet   map[string]any
	LastRuntime   map[string]any
	CurrentGateID int
	StartedAt     string

	mu      sync.Mutex
	process *exec.Cmd
	done    chan error
}

func New(cfg config.Config) (*Controller, error) {
	stateDir := filepath.Join(cfg.BaseDir, "state")
	device, err := sota.LoadOrCreateDevice(stateDir, cfg.HWID, cfg.DeviceName)
	if err != nil {
		return nil, err
	}
	client := sota.NewClient(cfg.AccessKey, device, cfg.APIBases, cfg.UserAgent, cfg.AcceptLanguage)
	runtimeDir := filepath.Join(cfg.BaseDir, "runtime")
	return &Controller{
		Config:      cfg,
		Client:      client,
		Device:      device,
		StateDir:    stateDir,
		RuntimeDir:  runtimeDir,
		RuleSetsDir: filepath.Join(cfg.BaseDir, "rule_sets"),
		RuntimePath: filepath.Join(runtimeDir, "sing-box.runtime.json"),
		SingBoxLog:  filepath.Join(runtimeDir, "sing-box.log"),
	}, nil
}

func (c *Controller) Profile(ctx context.Context) (map[string]any, error) {
	return c.Client.Profile(ctx)
}

func (c *Controller) Locations(ctx context.Context) ([]map[string]any, error) {
	return c.Client.Locations(ctx)
}

func (c *Controller) Render(ctx context.Context, gate any) (string, map[string]any, map[string]any, error) {
	locations, err := c.Locations(ctx)
	if err != nil {
		c.LastError = err.Error()
		return "", nil, nil, err
	}
	gateID, err := SelectGateID(locations, gate)
	if err != nil {
		c.LastError = err.Error()
		return "", nil, nil, err
	}
	snippet, err := c.Client.Connect(ctx, gateID)
	if err != nil {
		c.LastError = err.Error()
		return "", nil, nil, err
	}
	runtime, err := runtimecfg.Build(ctx, snippet, runtimecfg.Options{
		Mode:          c.Config.Mode,
		RuleSetsDir:   c.RuleSetsDir,
		UserAgent:     c.Config.UserAgent,
		LogLevel:      c.Config.LogLevel,
		ProxyListen:   c.Config.ProxyListen,
		CacheRuleSets: true,
	})
	if err != nil {
		c.LastError = err.Error()
		return "", nil, nil, err
	}
	if err := runtimecfg.WriteJSON(c.RuntimePath, runtime); err != nil {
		c.LastError = err.Error()
		return "", nil, nil, err
	}
	c.CurrentGateID = gateID
	c.LastSnippet = snippet
	c.LastRuntime = runtime
	c.LastError = ""
	return c.RuntimePath, snippet, runtime, nil
}

func (c *Controller) Start(ctx context.Context, gate any) (map[string]any, error) {
	c.mu.Lock()
	if c.process != nil && c.process.Process != nil && c.process.ProcessState == nil {
		_ = c.stopLocked()
	}
	c.mu.Unlock()

	path, _, _, err := c.Render(ctx, gate)
	if err != nil {
		return nil, err
	}
	bin, err := singbox.DiscoverOrDownload(ctx, singbox.Options{
		ExplicitBin: c.Config.SingBoxBin,
		Dir:         c.Config.SingBoxDir,
		Version:     c.Config.SingBoxVersion,
		UserAgent:   c.Config.UserAgent,
	})
	if err != nil {
		c.LastError = err.Error()
		return nil, err
	}
	checkOut, err := singbox.Check(ctx, bin, path)
	if err != nil {
		c.LastError = tail(checkOut, 2000)
		return nil, fmt.Errorf("sing-box check failed:\n%s", c.LastError)
	}
	if err := os.MkdirAll(filepath.Dir(c.SingBoxLog), 0o755); err != nil {
		c.LastError = err.Error()
		return nil, err
	}
	logFile, err := os.OpenFile(c.SingBoxLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		c.LastError = err.Error()
		return nil, err
	}
	_, _ = fmt.Fprintf(logFile, "\n=== sing-box start %s ===\n", time.Now().UTC().Format(time.RFC3339))
	_ = logFile.Close()

	cmd := singbox.Command(bin, path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.LastError = err.Error()
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		c.LastError = err.Error()
		return nil, err
	}
	go pumpProcessOutput(stdout, c.SingBoxLog)
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	time.Sleep(time.Second)
	select {
	case err := <-done:
		c.LastError = "sing-box exited immediately. Log: " + c.SingBoxLog
		tailText := singbox.TailFile(c.SingBoxLog, 4000)
		if tailText != "" {
			c.LastError += "\n" + tailText
		}
		if err != nil {
			c.LastError += "\n" + err.Error()
		}
		return nil, fmt.Errorf("%s", c.LastError)
	default:
	}

	c.mu.Lock()
	c.process = cmd
	c.done = done
	c.StartedAt = time.Now().UTC().Format(time.RFC3339)
	c.LastError = ""
	c.mu.Unlock()
	return map[string]any{
		"ok":             true,
		"pid":            cmd.Process.Pid,
		"gate_id":        c.CurrentGateID,
		"runtime_config": path,
		"sing_box_log":   c.SingBoxLog,
	}, nil
}

func (c *Controller) Stop() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopLocked()
}

func (c *Controller) stopLocked() map[string]any {
	if c.process == nil || c.process.Process == nil {
		c.process = nil
		c.done = nil
		c.StartedAt = ""
		return map[string]any{"ok": true, "stopped": false}
	}
	if c.process.ProcessState != nil {
		c.process = nil
		c.done = nil
		c.StartedAt = ""
		return map[string]any{"ok": true, "stopped": false}
	}
	_ = c.process.Process.Signal(os.Interrupt)
	done := c.done
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		_ = c.process.Process.Kill()
		<-done
	}
	c.process = nil
	c.done = nil
	c.StartedAt = ""
	return map[string]any{"ok": true, "stopped": true}
}

func (c *Controller) Reload(ctx context.Context) (map[string]any, error) {
	gate := c.CurrentGateID
	if gate == 0 {
		return c.Start(ctx, nil)
	}
	return c.Start(ctx, gate)
}

func (c *Controller) Status() map[string]any {
	c.mu.Lock()
	running := c.process != nil && c.process.Process != nil && c.process.ProcessState == nil
	pid := any(nil)
	if running {
		pid = c.process.Process.Pid
	}
	c.mu.Unlock()
	state := "disconnected"
	if running {
		state = "connected"
	}
	return map[string]any{
		"version":             config.AppVersion,
		"mode":                c.Config.Mode,
		"running":             running,
		"pid":                 pid,
		"connectionStartDate": c.StartedAt,
		"connectionState":     state,
		"gate_id":             nullIfZero(c.CurrentGateID),
		"runtime_config":      nullIfEmpty(c.RuntimePath),
		"sing_box_log":        c.SingBoxLog,
		"api_base":            nullIfEmpty(c.Client.ActiveBase),
		"last_error":          c.LastError,
		"device": map[string]any{
			"x_device_name": c.Device.DeviceName,
			"x_hwid":        RedactValue(c.Device.HWID),
		},
	}
}

func (c *Controller) RuntimeConfig(raw bool) (map[string]any, error) {
	var data map[string]any
	if c.LastRuntime != nil {
		data = c.LastRuntime
	} else if _, err := os.Stat(c.RuntimePath); err == nil {
		read, err := runtimecfg.ReadJSON(c.RuntimePath)
		if err != nil {
			return nil, err
		}
		data = read
	} else {
		data = map[string]any{}
	}
	if raw {
		return data, nil
	}
	return Redact(data).(map[string]any), nil
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

func pumpProcessOutput(pipe io.ReadCloser, logPath string) {
	defer pipe.Close()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[sing-box-log-pump-error] %v\n", err)
		return
	}
	defer logFile.Close()
	buf := make([]byte, 4096)
	for {
		n, err := pipe.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = logFile.Write(chunk)
			_, _ = fmt.Fprint(os.Stderr, "[sing-box] "+string(chunk))
		}
		if err != nil {
			return
		}
	}
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

func nullIfZero(i int) any {
	if i == 0 {
		return nil
	}
	return i
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
