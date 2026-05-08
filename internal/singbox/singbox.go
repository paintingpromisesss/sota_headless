package singbox

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"sota-headless/internal/httpclient"
)

type Options struct {
	ExplicitBin string
	Dir         string
	Version     string
	UserAgent   string
}

func DiscoverOrDownload(ctx context.Context, opts Options) (string, error) {
	if opts.ExplicitBin != "" {
		if exists(opts.ExplicitBin) {
			return opts.ExplicitBin, nil
		}
		return "", fmt.Errorf("SING_BOX_BIN does not exist: %s", opts.ExplicitBin)
	}
	if found, err := exec.LookPath(binaryName()); err == nil {
		return found, nil
	}
	for _, candidate := range localCandidates(opts.Dir) {
		if exists(candidate) {
			return candidate, nil
		}
	}
	if opts.Dir == "" {
		return "", fmt.Errorf("sing-box binary not found and SING_BOX_DIR is empty")
	}
	if opts.Version == "" {
		return "", fmt.Errorf("sing-box binary not found and SING_BOX_VERSION is empty")
	}
	return download(ctx, opts)
}

func Check(ctx context.Context, bin, configPath string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, "check", "-c", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("sing-box check failed: %w", err)
	}
	return string(out), nil
}

func Command(bin, configPath string) *exec.Cmd {
	return exec.Command(bin, "run", "-c", configPath)
}

func TailFile(path string, maxChars int64) string {
	if maxChars <= 0 {
		maxChars = 12000
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	offset := info.Size() - maxChars
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return fmt.Sprintf("<failed to read %s: %v>", path, err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Sprintf("<failed to read %s: %v>", path, err)
	}
	return string(data)
}

func AssetName(version, goos, goarch string) (string, error) {
	cleanVersion := strings.TrimPrefix(version, "v")
	assetOS := goos
	assetArch := goarch
	switch goarch {
	case "amd64", "386", "arm64":
	case "arm":
		assetArch = "armv7"
	default:
		return "", fmt.Errorf("unsupported architecture for sing-box download: %s/%s", goos, goarch)
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	switch goos {
	case "linux", "darwin", "windows", "freebsd":
	default:
		return "", fmt.Errorf("unsupported OS for sing-box download: %s/%s", goos, goarch)
	}
	return fmt.Sprintf("sing-box-%s-%s-%s%s", cleanVersion, assetOS, assetArch, ext), nil
}

func download(ctx context.Context, opts Options) (string, error) {
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return "", err
	}
	asset, err := AssetName(opts.Version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	version := opts.Version
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	url := fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/%s/%s", version, asset)
	tmp := filepath.Join(opts.Dir, asset+".download")
	if err := downloadFile(ctx, url, tmp, opts.UserAgent); err != nil {
		return "", err
	}
	defer os.Remove(tmp)
	if strings.HasSuffix(asset, ".zip") {
		if err := unpackZip(tmp, opts.Dir); err != nil {
			return "", err
		}
	} else {
		if err := unpackTarGz(tmp, opts.Dir); err != nil {
			return "", err
		}
	}
	for _, candidate := range localCandidates(opts.Dir) {
		if exists(candidate) {
			_ = os.Chmod(candidate, 0o755)
			return candidate, nil
		}
	}
	return "", fmt.Errorf("downloaded %s but sing-box binary was not found after unpack", asset)
}

func downloadFile(ctx context.Context, source, dst, userAgent string) error {
	client := httpclient.New(httpclient.WithTimeout(httpclient.DefaultDownloadTime))
	return client.DownloadFile(ctx, source, map[string]string{"User-Agent": userAgent}, dst)
}

func unpackTarGz(path, dstDir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(header.Name) != binaryName() {
			continue
		}
		target := filepath.Join(dstDir, binaryName())
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
}

func unpackZip(path, dstDir string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, file := range zr.File {
		if filepath.Base(file.Name) != binaryName() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, binaryName())
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeInErr := rc.Close()
		closeOutErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInErr != nil {
			return closeInErr
		}
		return closeOutErr
	}
	return nil
}

func localCandidates(dir string) []string {
	if dir == "" {
		return nil
	}
	return []string{filepath.Join(dir, binaryName())}
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "sing-box.exe"
	}
	return "sing-box"
}

func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
