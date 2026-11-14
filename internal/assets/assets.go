package assets

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	root "github.com/ryangerardwilson/tesseract"
)

func DefaultAppDir(home string) string {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil || home == "" {
			home = "."
		}
	}
	return filepath.Join(home, ".tesseract", "app")
}

func Write(targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(root.Assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(targetDir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := root.Assets.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func renderPlaceholders(data []byte, indexKey, publicLauncher string) []byte {
	rendered := strings.ReplaceAll(string(data), "__TMUX_INDEX_KEY__", indexKey)
	rendered = strings.ReplaceAll(rendered, "__TM_PUBLIC_LAUNCHER__", publicLauncher)
	return []byte(rendered)
}

func RenderTmuxConfig(targetPath, indexKey, publicLauncher string) error {
	configDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	for _, stale := range []string{
		filepath.Join(configDir, "tmux.conf.d"),
		filepath.Join(configDir, "scripts"),
		filepath.Join(configDir, "even-horizontal-panes"),
	} {
		if err := os.RemoveAll(stale); err != nil {
			return err
		}
	}
	return fs.WalkDir(root.Assets, "tmux", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "tmux" {
			return nil
		}
		rel := strings.TrimPrefix(path, "tmux/")
		target := filepath.Join(configDir, filepath.FromSlash(rel))
		if rel == "tmux.conf" {
			target = targetPath
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := root.Assets.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if strings.HasPrefix(rel, "scripts/") {
			mode = 0o755
		}
		return os.WriteFile(target, renderPlaceholders(data, indexKey, publicLauncher), mode)
	})
}
