package singbox

import "testing"

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
