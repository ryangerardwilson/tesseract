package player

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFFPlayArgsAlwaysApplyAnalogFilter(t *testing.T) {
	got := ffplayArgs("/music/track.mp3", 0)
	want := []string{
		"-nodisp",
		"-autoexit",
		"-loglevel",
		"quiet",
		"-af",
		analogPlaybackFilter,
		"/music/track.mp3",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ffplayArgs() = %#v, want %#v", got, want)
	}
}

func TestFFPlayArgsKeepAnalogFilterWhenSeeking(t *testing.T) {
	got := ffplayArgs("/music/track.mp3", 83*time.Second)
	want := []string{
		"-nodisp",
		"-autoexit",
		"-loglevel",
		"quiet",
		"-ss",
		"83",
		"-af",
		analogPlaybackFilter,
		"/music/track.mp3",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ffplayArgs() = %#v, want %#v", got, want)
	}
}

func TestAnalogPlaybackFilterHasNoBypassMode(t *testing.T) {
	for _, filter := range []string{
		"highpass",
		"lowpass",
		"equalizer",
		"acompressor",
		"asoftclip",
		"alimiter",
	} {
		if !strings.Contains(analogPlaybackFilter, filter) {
			t.Fatalf("analogPlaybackFilter missing %q: %q", filter, analogPlaybackFilter)
		}
	}
}
