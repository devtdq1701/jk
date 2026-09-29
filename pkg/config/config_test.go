package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		CurrentContext: "prod",
		Contexts: map[string]Context{
			"prod": {
				URL:      "http://jenkins.example.com",
				Username: "admin",
				Token:    "secret123",
			},
		},
	}

	if err := cfg.Save(cfgFile); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	info, err := os.Stat(cfgFile)
	if err != nil {
		t.Fatalf("Stat() failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %#o", info.Mode().Perm())
	}

	loaded, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if loaded.CurrentContext != "prod" {
		t.Errorf("expected CurrentContext 'prod', got '%s'", loaded.CurrentContext)
	}
	ctx, ok := loaded.Contexts["prod"]
	if !ok || ctx.Token != "secret123" {
		t.Errorf("expected token 'secret123', got %+v", ctx)
	}

	// ResolveContext tests
	_, c, err := loaded.ResolveContext("")
	if err != nil || c.Username != "admin" {
		t.Errorf("ResolveContext default failed: %v", err)
	}

	_, _, err = loaded.ResolveContext("nonexistent")
	if err == nil {
		t.Errorf("expected error for nonexistent context, got nil")
	}
}
