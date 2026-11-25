package library

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func IsYouTubeURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "youtube.com" || host == "www.youtube.com" || host == "youtu.be"
}

func DownloadYouTubeAudio(playlistDir, filename, rawURL string) error {
	if !IsYouTubeURL(rawURL) {
		return fmt.Errorf("not a YouTube URL: %s", rawURL)
	}
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("missing target filename")
	}
	if filepath.Ext(filename) == "" {
		filename += ".mp3"
	}
	target := filepath.Join(playlistDir, filepath.Base(filename))
	if err := os.MkdirAll(playlistDir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command(
		"yt-dlp",
		"-x",
		"--audio-format",
		"mp3",
		"--audio-quality",
		"0",
		"--embed-thumbnail",
		"--add-metadata",
		"--embed-chapters",
		"--no-playlist",
		"-o",
		target,
		rawURL,
	)
	return cmd.Run()
}
