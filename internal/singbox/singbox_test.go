package singbox

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestAssetName(t *testing.T) {
	got, err := AssetName("v1.13.11", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	want := "sing-box-1.13.11-linux-amd64.tar.gz"
	if got != want {
		t.Fatalf("asset = %q, want %q", got, want)
	}
	got, err = AssetName("1.13.11", "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	want = "sing-box-1.13.11-windows-amd64.zip"
	if got != want {
		t.Fatalf("asset = %q, want %q", got, want)
	}
}

func TestUnpackTarGzWritesExecutableBinary(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "sing-box.tar.gz")
	writeTarGz(t, archivePath, "sing-box-1.13.11-linux-amd64/sing-box", "#!/bin/sh\n")
	dst := t.TempDir()

	if err := unpackTarGz(archivePath, dst); err != nil {
		t.Fatal(err)
	}

	assertExecutable(t, filepath.Join(dst, binaryName()))
}

func TestUnpackZipWritesExecutableBinary(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "sing-box.zip")
	writeZip(t, archivePath, "sing-box-1.13.11-windows-amd64/sing-box", "#!/bin/sh\n")
	dst := t.TempDir()

	if err := unpackZip(archivePath, dst); err != nil {
		t.Fatal(err)
	}

	assertExecutable(t, filepath.Join(dst, binaryName()))
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("%s mode = %v, want executable bit", path, info.Mode())
	}
}

func writeTarGz(t *testing.T, archivePath, name, body string) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, archivePath, name, body string) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	defer zw.Close()
	writer, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
}
