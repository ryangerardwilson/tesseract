package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListPlaylistsAndTracks(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "zeta.mp3"))
	mustWrite(t, filepath.Join(root, "ignore.txt"))
	if err := os.MkdirAll(filepath.Join(root, "Alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "Alpha", "two.flac"))
	mustWrite(t, filepath.Join(root, "Alpha", "one.mp3"))

	playlists := ListPlaylists(root)
	if len(playlists) != 2 || playlists[0].Name != "Alpha" || playlists[1].Name != "beta" {
		t.Fatalf("unexpected playlists: %#v", playlists)
	}

	rootTracks := ListRootTracks(root)
	if len(rootTracks) != 1 || rootTracks[0].Name != "zeta" {
		t.Fatalf("unexpected root tracks: %#v", rootTracks)
	}

	tracks := ScanPlaylistTracks(filepath.Join(root, "Alpha"))
	if len(tracks) != 2 || tracks[0].Name != "one" || tracks[1].Name != "two" {
		t.Fatalf("unexpected playlist tracks: %#v", tracks)
	}
}

func TestYouTubeURLDetection(t *testing.T) {
	for _, raw := range []string{"https://youtube.com/watch?v=x", "https://www.youtube.com/watch?v=x", "https://youtu.be/x"} {
		if !IsYouTubeURL(raw) {
			t.Fatalf("%q should be accepted", raw)
		}
	}
	if IsYouTubeURL("https://example.com/watch?v=x") {
		t.Fatal("example.com should not be accepted")
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
