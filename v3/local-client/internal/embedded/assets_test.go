package embedded

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedExtensionExtractsV3Manifest(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "extension")
	if err := ExtractExtension(destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "3.0.1" {
		t.Fatalf("unexpected embedded extension version %q", manifest.Version)
	}
}
