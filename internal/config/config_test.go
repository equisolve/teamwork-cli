package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempHome points os.UserHomeDir at a temp dir for the duration of t.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("TEAMWORK_URL", "")
	t.Setenv("TEAMWORK_TOKEN", "")
	return dir
}

func TestLoadSave_Roundtrip(t *testing.T) {
	home := withTempHome(t)

	cfg := &Config{URL: "https://x.teamwork.com", Token: "abcd"}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "teamwork", "config.yaml")); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != cfg.URL || got.Token != cfg.Token {
		t.Errorf("load mismatch: got %+v, want %+v", got, cfg)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	withTempHome(t)
	if err := Save(&Config{URL: "https://file.teamwork.com", Token: "filetoken"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEAMWORK_URL", "https://env.teamwork.com")
	t.Setenv("TEAMWORK_TOKEN", "envtoken")

	got, _ := Load()
	if got.URL != "https://env.teamwork.com" {
		t.Errorf("url = %q, want env override", got.URL)
	}
	if got.Token != "envtoken" {
		t.Errorf("token = %q, want env override", got.Token)
	}
}

func TestSet_KnownAndUnknown(t *testing.T) {
	withTempHome(t)
	if err := Set("url", "https://abc.teamwork.com"); err != nil {
		t.Fatal(err)
	}
	if err := Set("token", "xyz"); err != nil {
		t.Fatal(err)
	}
	if err := Set("wat", "?"); err == nil {
		t.Error("expected error for unknown key")
	}
	got, _ := Load()
	if got.URL != "https://abc.teamwork.com" || got.Token != "xyz" {
		t.Errorf("unexpected config after Set: %+v", got)
	}
}

func TestGet(t *testing.T) {
	withTempHome(t)
	_ = Save(&Config{URL: "https://x.teamwork.com", Token: "tok"})
	u, err := Get("url")
	if err != nil || u != "https://x.teamwork.com" {
		t.Errorf("Get(url) = %q, %v", u, err)
	}
	if _, err := Get("nonexistent"); err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestPath(t *testing.T) {
	withTempHome(t)
	p := Path()
	if filepath.Base(p) != "config.yaml" {
		t.Errorf("path base = %q, want config.yaml", filepath.Base(p))
	}
}
