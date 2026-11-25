package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPathUsesXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg"))
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	got := DefaultConfigPath()
	want := filepath.Join(tmp, "xdg", "r2d2", "musica", "config.json")
	if got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathUsesHomeConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", tmp)

	got := DefaultConfigPath()
	want := filepath.Join(tmp, ".config", "r2d2", "musica", "config.json")
	if got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func writeConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestAdoptLegacyConfigCopiesMusicaConfigOnce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	old := filepath.Join(root, "musica", "config.json")
	writeConfig(t, old, `{"music_dir": "/music/old"}`)
	writeConfig(t, filepath.Join(root, "rlc", "config.json"), `{"music_dir": "/music/rlc"}`)

	path := DefaultConfigPath()
	if err := AdoptLegacyConfig(path); err != nil {
		t.Fatal(err)
	}
	if got := readConfig(t, path); got != `{"music_dir": "/music/old"}` {
		t.Fatalf("adopted %q", got)
	}
	if err := SaveUserConfig(path, UserConfig{MusicDir: "/music/new"}); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, old, `{"music_dir": "/music/changed"}`)
	if err := AdoptLegacyConfig(path); err != nil {
		t.Fatal(err)
	}
	if got := LoadUserConfig(path).MusicDir; got != "/music/new" {
		t.Fatalf("new config MusicDir = %q", got)
	}
	if got := readConfig(t, old); got != `{"music_dir": "/music/changed"}` {
		t.Fatalf("old config was written: %q", got)
	}
}

func TestAdoptLegacyConfigFallsBackToRLC(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	writeConfig(t, filepath.Join(root, "rlc", "config.json"), `{"fps": 30}`)

	path := DefaultConfigPath()
	if err := AdoptLegacyConfig(path); err != nil {
		t.Fatal(err)
	}
	if got := LoadUserConfig(path).FPS; got != 30 {
		t.Fatalf("FPS = %d", got)
	}
	if _, err := os.Stat(filepath.Join(root, "musica", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("old musica path was written: %v", err)
	}
}

func TestAdoptLegacyConfigWithoutOldFileWritesNothing(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)

	path := DefaultConfigPath()
	if err := AdoptLegacyConfig(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config was written: %v", err)
	}
}

func TestSaveLoadAndBuildConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.json")
	musicDir := filepath.Join(tmp, "Music")
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := SaveUserConfig(path, UserConfig{MusicDir: musicDir, FPS: 3}); err != nil {
		t.Fatal(err)
	}

	loaded := LoadUserConfig(path)
	if loaded.MusicDir != musicDir {
		t.Fatalf("MusicDir = %q, want %q", loaded.MusicDir, musicDir)
	}
	cfg := BuildAppConfig("", nil, path)
	if cfg.MusicDir != musicDir {
		t.Fatalf("app MusicDir = %q, want %q", cfg.MusicDir, musicDir)
	}
	if cfg.FPS != 5 {
		t.Fatalf("FPS = %d, want floor 5", cfg.FPS)
	}
}

func TestEnsureConfigFileWritesBootstrap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "musica", "config.json")
	if err := EnsureConfigFile(path); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(BootstrapText()) {
		t.Fatalf("bootstrap body = %q", string(body))
	}
}
