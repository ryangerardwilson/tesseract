package visualizer

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	defaultSampleRate    = 11025
	defaultWindowSamples = 1024
	defaultBands         = 32
	// kickTop is the highest band center, in Hz, the kick is heard in: from
	// 40 Hz up through a kick drum's thump and its punch.
	kickTop = 170
	// kickFloor is the low-end rise, in band energy, that counts as a whole
	// kick when nothing has risen for a while: a 60 Hz tone about 52 dB under
	// full scale. A rise out of silence or dither stays far below it.
	kickFloor = 1e8
)

type bandBins struct {
	low    int
	center int
	high   int
}

type Analyzer struct {
	mu sync.Mutex

	ffmpegPath string
	cmd        *exec.Cmd
	done       chan struct{}

	sampleRate    int
	windowSamples int
	bands         int

	levels     []float64
	peaks      []float64
	bandPeaks  []float64
	noiseFloor []float64
	bins       []bandBins
	window     []float64
	tilt       []float64

	lowBaseline float64
	beatEnv     float64
	paused      bool

	// The stream is heard half a window at a time: pcm is the last whole
	// window, and halves counts the halves since the stream started.
	pcm    []int16
	halves int
	// The kick is heard in the lowest kickBands bands. kickHold is what is
	// left of their recent energy, kickBase is its slow mean, kickMean is the
	// mean of its recent rises, kickHit is the last window's hit, and thump
	// is the hardest kick since Thump last took it.
	kickBands int
	kickHold  float64
	kickBase  float64
	kickMean  float64
	kickHit   float64
	thump     float64
}

func New(bands int) *Analyzer {
	if bands <= 0 {
		bands = defaultBands
	}
	path, _ := exec.LookPath("ffmpeg")
	a := &Analyzer{
		ffmpegPath:    path,
		sampleRate:    defaultSampleRate,
		windowSamples: defaultWindowSamples,
		bands:         bands,
		levels:        make([]float64, bands),
		peaks:         make([]float64, bands),
		bandPeaks:     make([]float64, bands),
		noiseFloor:    make([]float64, bands),
		lowBaseline:   1e-6,
		window:        buildWindow(defaultWindowSamples),
		tilt:          buildTilt(bands),
		pcm:           make([]int16, defaultWindowSamples),
	}
	a.bins = a.buildBandBins()
	a.kickBands = a.countKickBands()
	a.resetShape()
	return a
}

func (a *Analyzer) Available() bool {
	return a.ffmpegPath != ""
}

func (a *Analyzer) Start(path string, position time.Duration) error {
	a.Stop()
	if a.ffmpegPath == "" {
		return nil
	}

	args := []string{"-v", "error", "-re"}
	if position > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", position.Seconds()))
	}
	args = append(args,
		"-i", path,
		"-f", "s16le",
		"-ac", "1",
		"-ar", strconv.Itoa(a.sampleRate),
		"-",
	)

	cmd := exec.Command(a.ffmpegPath, args...)
	cmd.Stdin = nil
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan struct{})
	a.mu.Lock()
	a.cmd = cmd
	a.done = done
	a.paused = false
	a.resetShapeLocked()
	a.mu.Unlock()

	go a.run(cmd, stdout, done)
	return nil
}

func (a *Analyzer) Stop() {
	a.mu.Lock()
	cmd := a.cmd
	done := a.done
	a.cmd = nil
	a.done = nil
	a.paused = false
	a.fadeLocked(0.65)
	a.thump = 0
	a.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	if done != nil {
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
	}
}

func (a *Analyzer) SetPaused(paused bool) {
	a.mu.Lock()
	cmd := a.cmd
	if cmd == nil || cmd.Process == nil {
		a.paused = false
		a.mu.Unlock()
		return
	}
	if a.paused == paused {
		a.mu.Unlock()
		return
	}
	a.paused = paused
	a.mu.Unlock()

	signal := syscall.SIGCONT
	if paused {
		signal = syscall.SIGSTOP
	}
	_ = syscall.Kill(-cmd.Process.Pid, signal)
}

func (a *Analyzer) Levels() []float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]float64(nil), a.levels...)
}

func (a *Analyzer) Peaks() []float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]float64(nil), a.peaks...)
}

// Thump is the hardest kick heard since the last call, from 0 to 1, and
// starts the next count from 0. A frame that takes it carries the strongest
// low-end hit since the frame before, however many windows came between, not
// what is left of it when the frame is built.
func (a *Analyzer) Thump() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	thump := a.thump
	a.thump = 0
	return thump
}

// Feed hears samples, mono at the analyzer's rate, as the stream does, half a
// window at a time; a part half at the end is dropped. It does nothing while
// a stream runs.
func (a *Analyzer) Feed(samples []int16) {
	half := a.windowSamples / 2
	for half > 0 && len(samples) >= half {
		if !a.hear(nil, samples[:half]) {
			return
		}
		samples = samples[half:]
	}
}

func (a *Analyzer) run(cmd *exec.Cmd, stdout io.Reader, done chan struct{}) {
	defer close(done)
	defer func() { _ = cmd.Wait() }()

	half := make([]int16, a.windowSamples/2)
	buf := make([]byte, len(half)*2)

	for {
		if _, err := io.ReadFull(stdout, buf); err != nil {
			break
		}
		for i := range half {
			half[i] = int16(binary.LittleEndian.Uint16(buf[i*2 : i*2+2]))
		}
		if !a.hear(cmd, half) {
			break
		}
	}

	a.mu.Lock()
	if a.cmd == cmd {
		a.cmd = nil
		a.done = nil
		a.paused = false
		a.fadeLocked(0.8)
	}
	a.mu.Unlock()
}

// hear takes the next half window of the stream. From the second half on,
// each half closes a window that overlaps the one before by half. Every
// window is heard for a kick, so a kick that lands across the edge of two
// windows is still heard whole, and every other window, a whole window after
// the last, also moves the levels. It reports false once cmd is no longer the
// analyzer's stream.
func (a *Analyzer) hear(cmd *exec.Cmd, half []int16) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cmd != cmd {
		return false
	}
	copy(a.pcm, a.pcm[len(half):])
	copy(a.pcm[len(a.pcm)-len(half):], half)
	a.halves++
	if a.halves < 2 {
		return true
	}
	raw := a.bandEnergies(a.pcm)
	a.thump = math.Max(a.thump, a.kick(raw))
	if a.halves%2 == 0 {
		a.moveLevelsLocked(a.shapeLevels(raw))
	}
	return true
}

// moveLevelsLocked moves each level toward its target, faster up than down,
// and lets each peak fall slowly behind its level.
func (a *Analyzer) moveLevelsLocked(targets []float64) {
	for i := 0; i < a.bands; i++ {
		previous := a.levels[i]
		target := targets[i]
		attack := 0.65
		release := 0.10
		hold := 0.90
		if i < minInt(6, a.bands) {
			attack = 0.72
			release = 0.12
			hold = 0.88
		}
		if target >= previous {
			a.levels[i] = previous*(1-attack) + target*attack
		} else {
			a.levels[i] = previous*hold + target*release
		}
		if a.levels[i] >= a.peaks[i] {
			a.peaks[i] = a.levels[i]
		} else {
			a.peaks[i] *= 0.965
		}
	}
}

// kick is how hard the low end hit in this window, from 0 to 1. It is the
// beat flux, the low end's energy above its baseline. The baseline is the
// louder of the slow mean and what is left of the windows before, a peak that
// falls by 30% each half window, so a held tone hits once, as it starts. A
// rise counts in full when it stands well clear of the mean of the recent
// rises, so each kick of a song reads near 1 however loud the song is. Only
// what a window adds to the hit of the window before counts, so two windows
// that overlap hear one kick once.
func (a *Analyzer) kick(raw []float64) float64 {
	count := minInt(a.kickBands, len(raw))
	low := 0.0
	for i := 0; i < count; i++ {
		low += raw[i] * a.tilt[i]
	}
	low /= float64(maxInt(1, count))
	a.kickBase = a.kickBase*0.988 + low*0.012
	rise := math.Max(0, low-math.Max(a.kickHold, a.kickBase)*1.12)
	a.kickHold = math.Max(low, a.kickHold*0.7)
	mean := a.kickMean
	a.kickMean = mean*0.95 + rise*0.05
	hit := clamp((rise-2*mean)/(4*mean+kickFloor), 0, 1)
	added := math.Max(0, hit-a.kickHit)
	a.kickHit = hit
	return added
}

// countKickBands is how many of the lowest bands lie under kickTop, at least
// one.
func (a *Analyzer) countKickBands() int {
	count := 0
	for _, frequency := range a.bandFrequencies() {
		if frequency < kickTop {
			count++
		}
	}
	return maxInt(1, count)
}

func (a *Analyzer) resetShape() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resetShapeLocked()
}

func (a *Analyzer) resetShapeLocked() {
	for i := 0; i < a.bands; i++ {
		a.levels[i] = 0
		a.peaks[i] = 0
		a.bandPeaks[i] = 1e-6
		a.noiseFloor[i] = 0
	}
	a.lowBaseline = 1e-6
	a.beatEnv = 0
	clear(a.pcm)
	a.halves = 0
	a.kickHold = 0
	a.kickBase = 0
	a.kickMean = 0
	a.kickHit = 0
	a.thump = 0
}

func (a *Analyzer) fadeLocked(multiplier float64) {
	for i := 0; i < a.bands; i++ {
		a.levels[i] *= multiplier
		a.peaks[i] *= multiplier
	}
}

func (a *Analyzer) buildBandBins() []bandBins {
	frequencies := a.bandFrequencies()
	maxBin := maxInt(2, a.windowSamples/2-1)
	bins := make([]bandBins, 0, len(frequencies))
	for _, frequency := range frequencies {
		k := int(0.5 + (float64(a.windowSamples)*frequency)/float64(a.sampleRate))
		k = minInt(maxBin, maxInt(1, k))
		bins = append(bins, bandBins{
			low:    maxInt(1, k-1),
			center: k,
			high:   minInt(maxBin, k+1),
		})
	}
	return bins
}

func (a *Analyzer) bandFrequencies() []float64 {
	low := 40.0
	high := math.Min(5000, float64(a.sampleRate)/2-50)
	if a.bands <= 1 {
		return []float64{(low + high) / 2}
	}

	out := make([]float64, 0, a.bands)
	ratio := math.Pow(high/low, 1/float64(a.bands-1))
	for i := 0; i < a.bands; i++ {
		out = append(out, low*math.Pow(ratio, float64(i)))
	}
	return out
}

func (a *Analyzer) bandEnergies(samples []int16) []float64 {
	energies := make([]float64, 0, len(a.bins))
	for _, bins := range a.bins {
		p0 := a.goertzelPower(samples, bins.low)
		p1 := a.goertzelPower(samples, bins.center)
		p2 := a.goertzelPower(samples, bins.high)
		energies = append(energies, math.Max(0, p0*0.22+p1*0.56+p2*0.22))
	}
	return energies
}

func (a *Analyzer) shapeLevels(raw []float64) []float64 {
	if len(raw) == 0 {
		return make([]float64, a.bands)
	}

	normed := make([]float64, a.bands)
	for i := 0; i < a.bands; i++ {
		energy := raw[i] * a.tilt[i]

		floor := a.noiseFloor[i]*0.997 + energy*0.003
		a.noiseFloor[i] = floor
		gated := math.Max(0, energy-floor*1.10)

		bandPeak := math.Max(a.bandPeaks[i]*0.992, math.Max(gated, 1e-6))
		a.bandPeaks[i] = bandPeak

		x := gated / bandPeak
		compressed := math.Log1p(x*10) / math.Log1p(10)
		normed[i] = clamp(math.Pow(compressed, 0.82), 0, 1)
	}

	lowCount := minInt(5, len(raw))
	lowEnergy := 0.0
	for i := 0; i < lowCount; i++ {
		lowEnergy += raw[i] * a.tilt[i]
	}
	lowEnergy /= float64(maxInt(1, lowCount))

	a.lowBaseline = a.lowBaseline*0.988 + lowEnergy*0.012
	beatFlux := math.Max(0, lowEnergy-a.lowBaseline*1.12)
	beatDrive := math.Min(0.72, beatFlux/(a.lowBaseline*1.85+1e-6))
	a.beatEnv = math.Max(a.beatEnv*0.84, beatDrive)

	for i := 0; i < minInt(6, len(normed)); i++ {
		lift := a.beatEnv * (0.28 - 0.04*float64(i))
		normed[i] = math.Min(1, normed[i]+math.Max(0, lift))
	}
	if a.beatEnv > 0.72 {
		for i := 0; i < minInt(3, len(normed)); i++ {
			floor := 0.44 - 0.06*float64(i)
			normed[i] = math.Max(normed[i], floor*a.beatEnv)
		}
	}

	if len(normed) < 3 {
		return normed
	}
	blurred := make([]float64, len(normed))
	blurred[0] = normed[0]
	for i := 1; i < len(normed)-1; i++ {
		blurred[i] = normed[i-1]*0.22 + normed[i]*0.56 + normed[i+1]*0.22
	}
	blurred[len(normed)-1] = normed[len(normed)-1]
	return blurred
}

func (a *Analyzer) goertzelPower(samples []int16, k int) float64 {
	omega := (2 * math.Pi * float64(k)) / float64(a.windowSamples)
	coeff := 2 * math.Cos(omega)
	sPrev := 0.0
	sPrev2 := 0.0
	for i, sample := range samples {
		weighted := float64(sample) * a.window[i]
		s := weighted + coeff*sPrev - sPrev2
		sPrev2 = sPrev
		sPrev = s
	}
	return sPrev2*sPrev2 + sPrev*sPrev - coeff*sPrev*sPrev2
}

func buildWindow(samples int) []float64 {
	if samples <= 1 {
		return []float64{1}
	}
	out := make([]float64, samples)
	for i := 0; i < samples; i++ {
		out[i] = 0.5 - 0.5*math.Cos((2*math.Pi*float64(i))/float64(samples-1))
	}
	return out
}

func buildTilt(bands int) []float64 {
	if bands <= 1 {
		return []float64{1}
	}
	out := make([]float64, bands)
	for i := 0; i < bands; i++ {
		position := float64(i) / float64(bands-1)
		out[i] = 1.35 - 0.40*position
	}
	return out
}

func clamp(value, low, high float64) float64 {
	return math.Min(high, math.Max(low, value))
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
