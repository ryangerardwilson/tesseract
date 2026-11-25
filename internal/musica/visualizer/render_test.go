package visualizer

import (
	"math"
	"strings"
	"testing"
)

func TestRenderKeepsRequestedShape(t *testing.T) {
	lines := Render(12, 4, []float64{0, 0.5, 1}, []float64{0, 0.75, 1})
	if len(lines) != 4 {
		t.Fatalf("len(lines) = %d, want 4", len(lines))
	}
	for i, line := range lines {
		if len(line) != 12 {
			t.Fatalf("len(lines[%d]) = %d, want 12", i, len(line))
		}
	}
}

func TestRenderShowsPeaks(t *testing.T) {
	lines := Render(8, 4, []float64{0.2, 0.2, 0.2, 0.2}, []float64{1, 1, 1, 1})
	if !strings.ContainsAny(lines[0], "*@") {
		t.Fatalf("top line = %q, want high-energy spark markers", lines[0])
	}
}

func TestRenderCreatesMirroredPulseField(t *testing.T) {
	lines := Render(10, 7, []float64{0.8, 0.6, 0.4, 0.6, 0.8}, nil)
	topEnergy := strings.TrimSpace(strings.Join(lines[:3], ""))
	bottomEnergy := strings.TrimSpace(strings.Join(lines[4:], ""))
	if topEnergy == "" || bottomEnergy == "" {
		t.Fatalf("expected mirrored field energy, got %#v", lines)
	}
}

func TestRenderFrameAnimatesMatrixRain(t *testing.T) {
	levels := []float64{0.7, 0.4, 0.8, 0.5, 0.9, 0.3}
	first := strings.Join(RenderFrame(24, 10, levels, nil, 1), "\n")
	next := strings.Join(RenderFrame(24, 10, levels, nil, 2), "\n")
	if first == next {
		t.Fatal("expected matrix rain to change between frames")
	}
}

func TestRenderCellsMixesRainGrayTones(t *testing.T) {
	cells := RenderCells(48, 18, []float64{0.6, 0.8, 0.5, 0.7, 0.9}, nil, 5)
	seen := map[CellTone]bool{}
	for _, row := range cells {
		for _, cell := range row {
			if isRainTone(cell.Tone) {
				seen[cell.Tone] = true
			}
		}
	}
	if len(seen) < 3 {
		t.Fatalf("rain should mix grayscale tones, got %v", seen)
	}
}

func TestRenderCellsBrightensRainWithMusicIntensity(t *testing.T) {
	low := RenderCells(48, 18, []float64{0.12, 0.08, 0.10, 0.09, 0.11}, nil, 5)
	high := RenderCells(48, 18, []float64{0.82, 0.88, 0.84, 0.90, 0.86}, nil, 5)

	lowBright := countBrightRainTones(low)
	highBright := countBrightRainTones(high)
	if highBright <= lowBright {
		t.Fatalf("high-intensity rain should have more bright gray tones: low=%d high=%d", lowBright, highBright)
	}
}

func TestRenderCellsLetsMusicPeaksReachWhite(t *testing.T) {
	cells := RenderCells(48, 18, []float64{0.72, 0.86, 0.78, 0.92, 0.84}, nil, 6)
	bright := countRainTone(cells, ToneRainBright)
	if bright < 6 {
		t.Fatalf("music peaks should reach white rain tone more easily: bright=%d", bright)
	}
}

func TestRenderCellsWithIntensityEasesRainDensity(t *testing.T) {
	levels := []float64{0.82, 0.88, 0.84, 0.90, 0.86}
	low := RenderCellsWithIntensity(48, 18, levels, nil, 6, 0.08)
	full := RenderCellsWithIntensity(48, 18, levels, nil, 6, 1)

	lowRain := countRainTones(low)
	fullRain := countRainTones(full)
	if fullRain == 0 {
		t.Fatalf("full intensity produced no rain tones")
	}
	if lowRain >= fullRain/4 {
		t.Fatalf("startup rain should ease in gently: low=%d full=%d", lowRain, fullRain)
	}
}

func TestFibonacciMusicScaleUsesFibonacciSteps(t *testing.T) {
	cases := []struct {
		value float64
		want  float64
	}{
		{value: 0.0, want: 0.0 / 13.0},
		{value: 1.0 / 6.0, want: 1.0 / 13.0},
		{value: 2.0 / 6.0, want: 2.0 / 13.0},
		{value: 3.0 / 6.0, want: 3.0 / 13.0},
		{value: 4.0 / 6.0, want: 5.0 / 13.0},
		{value: 5.0 / 6.0, want: 8.0 / 13.0},
		{value: 1.0, want: 13.0 / 13.0},
	}
	for _, tc := range cases {
		got := fibonacciMusicScale(tc.value)
		if math.Abs(got-tc.want) > 0.000001 {
			t.Fatalf("fibonacciMusicScale(%.3f) = %.6f, want %.6f", tc.value, got, tc.want)
		}
	}
}

func TestDrawBlackHoleDarkensRainThroughEventHorizon(t *testing.T) {
	const (
		width  = 49
		height = 17
	)
	canvas := make([][]byte, height)
	tones := make([][]CellTone, height)
	for y := range canvas {
		canvas[y] = []byte(strings.Repeat(" ", width))
		tones[y] = make([]CellTone, width)
	}
	centerX := float64(width-1) / 2
	centerY := float64(height-1) / 2
	cx := int(centerX + 0.5)
	cy := int(centerY + 0.5)
	canvas[cy][cx] = '0'

	drawBlackHoleWithTones(canvas, tones, width, height, 0.50, 0.50, centerX, centerY, 4.8, 3)

	if canvas[cy][cx] != '0' {
		t.Fatalf("event-horizon rain = %q, want preserved rain glyph", canvas[cy][cx])
	}
	if tones[cy][cx] != ToneDark {
		t.Fatalf("event-horizon rain tone = %v, want ToneDark", tones[cy][cx])
	}
}

func TestRenderFrameDrawsCircularPhotonRings(t *testing.T) {
	lines := RenderFrame(49, 17, []float64{0.4, 0.5, 0.4, 0.5, 0.4}, nil, 4)
	centerY := len(lines) / 2
	centerX := len(lines[0]) / 2

	ringGlyphs := 0
	for y := centerY - 7; y <= centerY+7; y++ {
		for x := centerX - 7; x <= centerX+7; x++ {
			if y < 0 || y >= len(lines) || x < 0 || x >= len(lines[y]) {
				continue
			}
			dist := absInt(x-centerX) + absInt(y-centerY)
			if dist >= 5 && dist <= 9 && strings.ContainsRune("#@*+=", rune(lines[y][x])) {
				ringGlyphs++
			}
		}
	}
	if ringGlyphs < 14 {
		t.Fatalf("photon ring too faint: count=%d lines=%#v", ringGlyphs, lines)
	}
}

func TestDrawBlackHoleDrawsThreeConcentricRings(t *testing.T) {
	const (
		width  = 72
		height = 24
	)
	canvas := make([][]byte, height)
	for y := range canvas {
		canvas[y] = []byte(strings.Repeat(" ", width))
	}
	centerX := float64(width-1) / 2
	centerY := float64(height-1) / 2
	radius := 6.2
	bass := 0.45
	drawBlackHole(canvas, width, height, bass, 0.50, centerX, centerY, radius, 8)

	coreRadius := radius * (1.12 + bass*0.05)
	for index, ringRadius := range blackHoleRingRadii(coreRadius) {
		count := 0
		for y := range canvas {
			for x := range canvas[y] {
				if !strings.ContainsRune("#@*+=", rune(canvas[y][x])) {
					continue
				}
				dx := float64(x) - centerX
				dy := float64(y) - centerY
				if terminalDistance(dx, dy) >= ringRadius-0.85 && terminalDistance(dx, dy) <= ringRadius+0.85 {
					count++
				}
			}
		}
		if count < 4 {
			t.Fatalf("ring %d too faint: count=%d radius=%.2f canvas=%#v", index+1, count, ringRadius, canvas)
		}
	}
}

func TestDrawBlackHoleLetsRainPassThroughRings(t *testing.T) {
	const (
		width  = 72
		height = 24
	)
	canvas := make([][]byte, height)
	for y := range canvas {
		canvas[y] = []byte(strings.Repeat(" ", width))
	}
	centerX := float64(width-1) / 2
	centerY := float64(height-1) / 2
	cx := int(centerX + 0.5)
	cy := int(centerY + 0.5)
	radius := 6.2
	bass := 0.45
	coreRadius := radius * (1.12 + bass*0.05)
	ringX := cx + int(blackHoleRingRadii(coreRadius)[1]+0.5)
	canvas[cy][ringX] = 'R'

	drawBlackHole(canvas, width, height, bass, 0.50, centerX, centerY, radius, 8)

	if canvas[cy][ringX] != 'R' {
		t.Fatalf("rain glyph crossing ring was overwritten: got %q canvas=%#v", canvas[cy][ringX], canvas)
	}
}

func TestDrawBlackHoleKeepsEventHorizonDarkAndRingVisible(t *testing.T) {
	const (
		width  = 60
		height = 20
	)
	canvas := make([][]byte, height)
	for y := range canvas {
		canvas[y] = []byte(strings.Repeat(" ", width))
	}
	centerX := float64(width-1) / 2
	centerY := float64(height-1) / 2
	radius := 4.8
	drawBlackHole(canvas, width, height, 0.35, 0.45, centerX, centerY, radius, 6)

	center := canvas[height/2][width/2]
	if center != ' ' {
		t.Fatalf("center = %q, want dark event horizon", center)
	}
	ringGlyphs := 0
	for y := height/2 - 6; y <= height/2+6; y++ {
		for x := width/2 - 18; x <= width/2+18; x++ {
			if strings.ContainsRune("#@*+=", rune(canvas[y][x])) {
				ringGlyphs++
			}
		}
	}
	if ringGlyphs < 28 {
		t.Fatalf("black-hole ring too faint: count=%d canvas=%#v", ringGlyphs, canvas)
	}
}

func TestDrawBlackHoleCompensatesTerminalCellAspect(t *testing.T) {
	const (
		width  = 72
		height = 24
	)
	canvas := make([][]byte, height)
	for y := range canvas {
		canvas[y] = []byte(strings.Repeat(" ", width))
	}
	centerX := float64(width-1) / 2
	centerY := float64(height-1) / 2
	drawBlackHole(canvas, width, height, 0.45, 0.50, centerX, centerY, 6.2, 8)

	minX, maxX := width, -1
	minY, maxY := height, -1
	for y := range canvas {
		for x := range canvas[y] {
			if strings.ContainsRune("#@*+=", rune(canvas[y][x])) {
				minX = minInt(minX, x)
				maxX = maxInt(maxX, x)
				minY = minInt(minY, y)
				maxY = maxInt(maxY, y)
			}
		}
	}
	if maxX < minX || maxY < minY {
		t.Fatalf("black hole did not render visible disc/ring: canvas=%#v", canvas)
	}
	shapeWidth := maxX - minX + 1
	shapeHeight := maxY - minY + 1
	displayRatio := float64(shapeWidth) / (float64(shapeHeight) * terminalCellAspect)
	if displayRatio < 0.78 || displayRatio > 1.28 {
		t.Fatalf("black hole should display as a circle: width=%d height=%d displayRatio=%.2f canvas=%#v", shapeWidth, shapeHeight, displayRatio, canvas)
	}
}

func TestDrawBlackHoleAvoidsLongCentralSlit(t *testing.T) {
	const (
		width  = 72
		height = 24
	)
	canvas := make([][]byte, height)
	for y := range canvas {
		canvas[y] = []byte(strings.Repeat(" ", width))
	}
	centerX := float64(width-1) / 2
	centerY := float64(height-1) / 2
	drawBlackHole(canvas, width, height, 0.45, 0.50, centerX, centerY, 6.2, 8)

	centerRow := canvas[height/2]
	bright := 0
	for _, ch := range centerRow {
		if strings.ContainsRune("#@*+=", rune(ch)) {
			bright++
		}
	}
	if bright > 14 {
		t.Fatalf("black hole has too much bright central slit: count=%d row=%q canvas=%#v", bright, string(centerRow), canvas)
	}
}

func TestMatrixRainBendsAroundBlackHole(t *testing.T) {
	const (
		width  = 72
		height = 24
	)
	canvas := make([][]byte, height)
	for y := range canvas {
		canvas[y] = []byte(strings.Repeat(" ", width))
	}
	levels := make([]float64, width)
	for i := range levels {
		levels[i] = 0.72
	}
	centerX := float64(width-1) / 2
	centerY := float64(height-1) / 2
	drawMatrixRain(canvas, nil, levels, 0.55, 0.50, centerX, centerY, 6.2, 18, 8, 1)

	warpedRain := 0
	for y := height/2 - 5; y <= height/2+5; y++ {
		for x := width/2 - 14; x <= width/2+14; x++ {
			if strings.ContainsRune("#@*+", rune(canvas[y][x])) {
				warpedRain++
			}
		}
	}
	if warpedRain < 36 {
		t.Fatalf("black-hole rain interaction too faint: count=%d canvas=%#v", warpedRain, canvas)
	}
}

func TestRenderClampsTinyDimensions(t *testing.T) {
	lines := Render(0, 0, nil, nil)
	if len(lines) != 1 || len(lines[0]) != 1 {
		t.Fatalf("tiny render shape = %d x %d, want 1 x 1", len(lines), len(lines[0]))
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func isRainTone(tone CellTone) bool {
	switch tone {
	case ToneRainTrace, ToneRainDim, ToneRainSoft, ToneRainGlow, ToneRainBright:
		return true
	default:
		return false
	}
}

func countBrightRainTones(cells [][]Cell) int {
	count := 0
	for _, row := range cells {
		for _, cell := range row {
			if cell.Tone == ToneRainGlow || cell.Tone == ToneRainBright {
				count++
			}
		}
	}
	return count
}

func countRainTones(cells [][]Cell) int {
	count := 0
	for _, row := range cells {
		for _, cell := range row {
			if isRainTone(cell.Tone) {
				count++
			}
		}
	}
	return count
}

func countRainTone(cells [][]Cell, tone CellTone) int {
	count := 0
	for _, row := range cells {
		for _, cell := range row {
			if cell.Tone == tone {
				count++
			}
		}
	}
	return count
}
