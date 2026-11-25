package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const DefaultFPS = 20

type AppConfig struct {
	MusicDir     string
	FPS          int
	StartupTrack string
}

type UserConfig struct {
	MusicDir string `json:"music_dir,omitempty"`
	FPS      int    `json:"fps,omitempty"`
}

// DefaultConfigPath is the one path musica reads and writes. r2d2 keeps each
// feature's config under <config>/r2d2/<feature>/.
func DefaultConfigPath() string {
	return filepath.Join(configHome(), "r2d2", "musica", "config.json")
}

func configHome() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return ExpandPath(base)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".config")
}

func LoadUserConfig(path string) UserConfig {
	if path == "" {
		path = DefaultConfigPath()
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return UserConfig{}
	}
	var cfg UserConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return UserConfig{}
	}
	return cfg
}

func SaveUserConfig(path string, updates UserConfig) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	cfg := LoadUserConfig(path)
	if updates.MusicDir != "" {
		cfg.MusicDir = updates.MusicDir
	}
	if updates.FPS != 0 {
		cfg.FPS = updates.FPS
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func BootstrapText() []byte {
	return []byte("{\n  \"music_dir\": \"\",\n  \"fps\": 20\n}\n")
}

func EnsureConfigFile(path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, BootstrapText(), 0o644)
}

// AdoptLegacyConfig copies an old config to path once, when path is absent.
// It tries <config>/musica/config.json, then <config>/rlc/config.json, and
// leaves the old file in place.
func AdoptLegacyConfig(path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(path)))
	for _, name := range []string{"musica", "rlc"} {
		body, err := os.ReadFile(filepath.Join(root, name, "config.json"))
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, body, 0o644)
	}
	return nil
}

func BuildAppConfig(cliMusicDir string, cliFPS *int, configPath string) AppConfig {
	userCfg := LoadUserConfig(configPath)
	musicDir := ResolveMusicDir(cliMusicDir, userCfg.MusicDir)

	fps := userCfg.FPS
	if cliFPS != nil {
		fps = *cliFPS
	}
	if fps == 0 {
		fps = DefaultFPS
	} else if fps < 5 {
		fps = 5
	}

	return AppConfig{
		MusicDir: musicDir,
		FPS:      fps,
	}
}

func ResolveMusicDir(cliValue, fileValue string) string {
	if cliValue != "" {
		return AbsPath(cliValue)
	}
	if fileValue != "" {
		return AbsPath(fileValue)
	}
	return AbsPath("~/Music")
}

func AbsPath(path string) string {
	expanded := ExpandPath(path)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return expanded
	}
	return abs
}

func ExpandPath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" || len(path) > 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return os.ExpandEnv(path)
}
