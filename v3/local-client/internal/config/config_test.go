package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPublicPortAndRoutes(t *testing.T) {
	cfg := DefaultConfig()
	if !strings.Contains(cfg.Connection.DirectURL, ":21314/") {
		t.Fatalf("direct URL does not use port 21314: %s", cfg.Connection.DirectURL)
	}
	if cfg.Connection.CloudDialAddress != "moonkey.top:21314" {
		t.Fatalf("unexpected cloud address: %s", cfg.Connection.CloudDialAddress)
	}
	if cfg.Server.ListenAddress != "[::]:21314" {
		t.Fatalf("unexpected server listen address: %s", cfg.Server.ListenAddress)
	}
	if cfg.Relay.RemoteListenAddress != "0.0.0.0:21314" {
		t.Fatalf("unexpected relay listen address: %s", cfg.Relay.RemoteListenAddress)
	}
	if cfg.Server.TLS.Mode != "local_ca" {
		t.Fatalf("unexpected default TLS mode: %s", cfg.Server.TLS.Mode)
	}
}

func TestOwnerProfileDoesNotExportPrivateState(t *testing.T) {
	runtimeDir := t.TempDir()
	cfg := DefaultConfig()
	if err := cfg.EnableOwnerMode(runtimeDir); err != nil {
		t.Fatal(err)
	}
	cfg.Connection.TLSCAPEM = "test owner CA"
	profile := cfg.ClientProfile()
	if profile.AccessToken == "" {
		t.Fatal("owner profile is missing access token")
	}
	if profile.TLSCAPEM == "" || profile.Version != ProfileVersion {
		t.Fatal("owner profile is missing local CA trust")
	}

	friend := DefaultConfig()
	if err := friend.ApplyProfile(profile); err != nil {
		t.Fatal(err)
	}
	if friend.Server.Enabled || friend.Relay.Enabled {
		t.Fatal("client profile enabled server-only components")
	}
	if friend.Connection.SessionToken == cfg.Connection.SessionToken {
		t.Fatal("client profile reused the owner's session token")
	}
	if friend.Connection.TLSCAPEM != profile.TLSCAPEM {
		t.Fatal("client profile did not import local CA trust")
	}
}

func TestLoadConfigCreatesIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Connection.ClientInstanceID == "" || cfg.Connection.SessionToken == "" {
		t.Fatal("generated config is missing stable identity")
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Connection.SessionToken != cfg.Connection.SessionToken {
		t.Fatal("session identity changed after reload")
	}
}
