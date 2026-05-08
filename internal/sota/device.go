package sota

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

func LoadOrCreateDevice(stateDir, envHWID, envDeviceName string) (Device, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return Device{}, err
	}
	path := filepath.Join(stateDir, "device.json")
	var device Device
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &device); err != nil {
			return Device{}, err
		}
	} else if os.IsNotExist(err) {
		hwid, err := generateHWID()
		if err != nil {
			return Device{}, err
		}
		device = Device{
			HWID:       hwid,
			DeviceName: defaultDeviceName(),
			CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		if err := writeJSONAtomic(path, device); err != nil {
			return Device{}, err
		}
	} else {
		return Device{}, err
	}
	if strings.TrimSpace(envHWID) != "" {
		device.HWID = strings.TrimSpace(envHWID)
	}
	if strings.TrimSpace(envDeviceName) != "" {
		device.DeviceName = strings.TrimSpace(envDeviceName)
	}
	if device.HWID == "" || device.DeviceName == "" {
		return Device{}, fmt.Errorf("device.json is missing x_hwid or x_device_name")
	}
	return device, nil
}

func generateHWID() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	host, _ := os.Hostname()
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
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
