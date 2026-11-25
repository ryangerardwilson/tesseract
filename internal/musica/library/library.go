package library

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var supportedExts = map[string]bool{
	".mp3":  true,
	".flac": true,
	".wav":  true,
	".ogg":  true,
	".m4a":  true,
}

type Track struct {
	Path     string
	Name     string
	Duration time.Duration
}

type Playlist struct {
	Path string
	Name string
}

func IsSupportedTrack(path string) bool {
	return supportedExts[strings.ToLower(filepath.Ext(path))]
}

func DisplayName(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return strings.TrimSuffix(name, ext)
}

func ListPlaylists(root string) []Playlist {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	playlists := make([]Playlist, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		playlists = append(playlists, Playlist{
			Path: path,
			Name: entry.Name(),
		})
	}
	sort.Slice(playlists, func(i, j int) bool {
		return strings.ToLower(playlists[i].Name) < strings.ToLower(playlists[j].Name)
	})
	return playlists
}

func ListRootTracks(root string) []Track {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	tracks := make([]Track, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !IsSupportedTrack(entry.Name()) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		tracks = append(tracks, Track{Path: path, Name: DisplayName(path)})
	}
	sortTracks(tracks)
	return tracks
}

func ScanMusicFiles(root string) []Track {
	tracks := []Track{}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return tracks
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !IsSupportedTrack(path) {
			return nil
		}
		tracks = append(tracks, Track{Path: path, Name: DisplayName(path)})
		return nil
	})
	sortTracks(tracks)
	return tracks
}

func ScanPlaylistTracks(playlistDir string) []Track {
	return ScanMusicFiles(playlistDir)
}

func sortTracks(tracks []Track) {
	sort.Slice(tracks, func(i, j int) bool {
		left := strings.ToLower(filepath.Base(tracks[i].Path))
		right := strings.ToLower(filepath.Base(tracks[j].Path))
		if left == right {
			return strings.ToLower(tracks[i].Path) < strings.ToLower(tracks[j].Path)
		}
		return left < right
	})
}

func FormatDuration(duration time.Duration) string {
	if duration <= 0 {
		return "--:--"
	}
	total := int(duration.Round(time.Second).Seconds())
	minutes := total / 60
	seconds := total % 60
	return twoDigits(minutes) + ":" + twoDigits(seconds)
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
