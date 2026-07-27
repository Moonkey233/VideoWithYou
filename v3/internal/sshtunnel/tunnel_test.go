package sshtunnel

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsurePrivateKeyIsStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh", "id_ed25519")
	first, err := EnsurePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EnsurePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("SSH public key changed after reload")
	}
	if !strings.HasPrefix(first, "ssh-ed25519 ") {
		t.Fatalf("unexpected public key format: %s", first)
	}
}

func TestHostKeyTrustOnFirstUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host.pin")
	if err := verifyOrTrustHostKey(path, "SHA256:first"); err != nil {
		t.Fatal(err)
	}
	if err := verifyOrTrustHostKey(path, "SHA256:first"); err != nil {
		t.Fatal(err)
	}
	if err := verifyOrTrustHostKey(path, "SHA256:changed"); err == nil {
		t.Fatal("changed host key was accepted")
	}
}
