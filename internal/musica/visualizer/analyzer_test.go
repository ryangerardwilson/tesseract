package visualizer

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnalyzerReadsSpectrum(t *testing.T) {
	analyzer := New(8)
	if !analyzer.Available() {
		t.Skip("ffmpeg not available")
	}

	path := filepath.Join(t.TempDir(), "tone.wav")
	writeToneWAV(t, path)

	if err := analyzer.Start(path, 0); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer analyzer.Stop()

	deadline := time.Now().Add(1200 * time.Millisecond)
	for time.Now().Before(deadline) {
		for _, level := range analyzer.Levels() {
			if level > 0.02 {
				return
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatal("analyzer levels never moved")
}

func TestShapeLevelsKeepsBassTransientPresentButControlled(t *testing.T) {
	analyzer := New(12)

	quiet := make([]float64, 12)
	for i := 0; i < 10; i++ {
		_ = analyzer.shapeLevels(quiet)
	}

	thump := make([]float64, 12)
	for i := 0; i < 4; i++ {
		thump[i] = 12000
	}

	levels := analyzer.shapeLevels(thump)
	low := average(levels[:4])
	high := average(levels[7:])

	if low < 0.45 {
		t.Fatalf("low-band thump response = %.3f, want at least 0.45", low)
	}
	if low <= high+0.18 {
		t.Fatalf("low-band response %.3f should dominate high-band response %.3f", low, high)
	}
	if analyzer.beatEnv > 0.95 {
		t.Fatalf("beat envelope = %.3f, want controlled response below saturation", analyzer.beatEnv)
	}
}

func TestThumpKeepsTheHardestKickUntilAFrameTakesIt(t *testing.T) {
	analyzer := New(32)
	analyzer.Feed(make([]int16, 8*defaultWindowSamples))
	if got := analyzer.Thump(); got > 0.05 {
		t.Fatalf("thump over silence = %.3f, want near 0", got)
	}

	// A kick and then a quiet window both come before the next frame.
	kick := make([]float64, 2*defaultWindowSamples)
	addKick(kick, 0, 0.8)
	analyzer.Feed(samplesOf(kick))
	if got := analyzer.Thump(); got < 0.9 {
		t.Fatalf("thump after a kick and a quiet window = %.3f, want the kick's, near 1", got)
	}
	if got := analyzer.Thump(); got != 0 {
		t.Fatalf("thump taken again = %.3f, want 0", got)
	}
}

func TestHeldToneHitsOnlyAsItStartsAndAKickOnItStillHits(t *testing.T) {
	// A low tone starts after silence and holds, and a kick lands on it.
	song := make([]float64, 40*defaultWindowSamples)
	addTone(song, 82, 0.45)
	addKick(song, 30*defaultWindowSamples+300, 0.6)
	held := samplesOf(song)

	analyzer := New(32)
	analyzer.Feed(make([]int16, 4*defaultWindowSamples))
	_ = analyzer.Thump()
	frame := func(w int) float64 {
		analyzer.Feed(held[w*defaultWindowSamples : (w+1)*defaultWindowSamples])
		return analyzer.Thump()
	}
	if got := frame(0); got < 0.9 {
		t.Fatalf("thump as the tone starts = %.3f, want near 1", got)
	}
	for w := 1; w < 30; w++ {
		if got := frame(w); got > 0.05 {
			t.Fatalf("thump %d windows into a held tone = %.3f, want near 0", w, got)
		}
	}
	if got := frame(30); got < 0.9 {
		t.Fatalf("thump for a kick on the held tone = %.3f, want near 1", got)
	}
	for w := 31; w < 40; w++ {
		if got := frame(w); got > 0.05 {
			t.Fatalf("thump %d windows after the kick = %.3f, want near 0", w-30, got)
		}
	}
}

func TestOverlappingWindowsHearAKickOnce(t *testing.T) {
	song := make([]float64, 16*defaultWindowSamples)
	addKick(song, 10*defaultWindowSamples+defaultWindowSamples/4, 0.6)
	samples := samplesOf(song)
	half := defaultWindowSamples / 2

	analyzer := New(32)
	analyzer.Feed(samples[:8*defaultWindowSamples])
	_ = analyzer.Thump()
	hits := 0
	for at := 8 * defaultWindowSamples; at < len(samples); at += half {
		analyzer.Feed(samples[at : at+half])
		switch got := analyzer.Thump(); {
		case got >= 0.9:
			hits++
		case got > 0.05:
			t.Fatalf("thump %.3f half %d windows in, want near 0 or near 1", got, (at-8*defaultWindowSamples)/half)
		}
	}
	if hits != 1 {
		t.Fatalf("a kick heard by overlapping windows hit %d times, want once", hits)
	}
}

func TestKickNearAWindowsEndIsHeardHalfAWindowLater(t *testing.T) {
	// A bass line holds under a kick that lands an eighth of a window before
	// a window ends, where that window hardly hears it.
	song := make([]float64, 12*defaultWindowSamples)
	addTone(song, 55, 0.2)
	addKick(song, 8*defaultWindowSamples+7*defaultWindowSamples/8, 0.6)
	samples := samplesOf(song)

	analyzer := New(32)
	analyzer.Feed(samples[:8*defaultWindowSamples])
	_ = analyzer.Thump()
	analyzer.Feed(samples[8*defaultWindowSamples : 9*defaultWindowSamples+defaultWindowSamples/2])
	if got := analyzer.Thump(); got < 0.9 {
		t.Fatalf("thump half a window into the next window = %.3f, want the kick, near 1", got)
	}
}

// addKick adds a kick drum at sample at: a low tone that falls from about
// 140 Hz to 50 Hz as it dies away.
func addKick(samples []float64, at int, level float64) {
	for i := at; i < len(samples); i++ {
		t := float64(i-at) / defaultSampleRate
		samples[i] += level * math.Sin(2*math.Pi*(50*t+3*(1-math.Exp(-30*t)))) * math.Exp(-18*t)
	}
}

func addTone(samples []float64, frequency, level float64) {
	for i := range samples {
		samples[i] += level * math.Sin(2*math.Pi*frequency*float64(i)/defaultSampleRate)
	}
}

func samplesOf(signal []float64) []int16 {
	out := make([]int16, len(signal))
	for i, value := range signal {
		out[i] = int16(32000 * clamp(value, -1, 1))
	}
	return out
}

func writeToneWAV(t *testing.T, path string) {
	t.Helper()

	const (
		sampleRate    = 11025
		duration      = 600 * time.Millisecond
		frequency     = 440.0
		bitsPerSample = 16
		channels      = 1
	)

	sampleCount := int(duration.Seconds() * sampleRate)
	dataSize := sampleCount * channels * bitsPerSample / 8

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*channels*bitsPerSample/8))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(channels*bitsPerSample/8))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	for i := 0; i < sampleCount; i++ {
		phase := 2 * math.Pi * frequency * float64(i) / sampleRate
		sample := int16(math.Sin(phase) * 18000)
		_ = binary.Write(&buf, binary.LittleEndian, sample)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}
}
