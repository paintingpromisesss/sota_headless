package sota

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Device struct {
	HWID       string `json:"x_hwid"`
	DeviceName string `json:"x_device_name"`
	CreatedAt  string `json:"created_at,omitempty"`
}

var (
	ErrInvalidDevice = errors.New("device.json is missing x_hwid or x_device_name")
)

func LoadOrCreateDevice(stateDir, envHWID, envDeviceName string) (Device, error) {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return Device{}, fmt.Errorf("failed to create state directory: %w", err)
	}
	path := filepath.Join(stateDir, "device.json")
	var device Device
	//nolint:gosec // path forms internally
	data, err := os.ReadFile(path)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		hwid, hwidErr := generateHWID()
		if hwidErr != nil {
			return Device{}, fmt.Errorf("failed to generate HWID: %w", hwidErr)
		}
		device = Device{
			HWID:       hwid,
			DeviceName: defaultDeviceName(),
			CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		if jsonErr := writeJSONAtomic(path, device); jsonErr != nil {
			return Device{}, fmt.Errorf("failed to write device.json: %w", jsonErr)
		}
	} else if err != nil {
		return Device{}, fmt.Errorf("failed to read device.json: %w", err)
	}

	if err = json.Unmarshal(data, &device); err != nil {
		return Device{}, fmt.Errorf("failed to unmarshal device.json: %w", err)
	}

	if strings.TrimSpace(envHWID) != "" {
		device.HWID = strings.TrimSpace(envHWID)
	}
	if strings.TrimSpace(envDeviceName) != "" {
		device.DeviceName = strings.TrimSpace(envDeviceName)
	}
	if device.HWID == "" || device.DeviceName == "" {
		return Device{}, ErrInvalidDevice
	}
	return device, nil
}

func generateHWID() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}
	seed := strings.Join([]string{
		"sota-headless",
		host,
		runtime.GOOS,
		runtime.GOARCH,
		hex.EncodeToString(random[:]),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:]), nil
}

func defaultDeviceName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "headless"
	}
	raw := fmt.Sprintf("%s_%s_%s", host, runtime.GOOS, runtime.GOARCH)
	var b strings.Builder
	for _, r := range raw {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	name := b.String()
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}

func HostHasNetwork() bool {
	_, err := net.LookupHost("localhost")
	return err == nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary JSON file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to rename temporary JSON file: %w", err)
	}
	return nil
}
