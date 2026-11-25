package visualizer

import (
	"math"
	"strings"
)

const (
	fieldGlyphs        = " .,:-=+*#%@"
	matrixGlyphs       = "0123456789"
	terminalCellAspect = 2.05
)

type CellTone int

const (
	ToneNormal CellTone = iota
	ToneDark
	ToneRainTrace
	ToneRainDim
	ToneRainSoft
	ToneRainGlow
	ToneRainBright
)

type Cell struct {
	Glyph byte
	Tone  CellTone
}

func Render(width int, height int, levels []float64, peaks []float64) []string {
	return RenderFrame(width, height, levels, peaks, 0)
}

func RenderFrame(width int, height int, levels []float64, peaks []float64, frame int) []string {
	return cellLines(RenderCells(width, height, levels, peaks, frame))
}

func RenderFrameWithIntensity(width int, height int, levels []float64, peaks []float64, frame int, intensityScale float64) []string {
	return cellLines(RenderCellsWithIntensity(width, height, levels, peaks, frame, intensityScale))
}

func RenderCells(width int, height int, levels []float64, peaks []float64, frame int) [][]Cell {
	return RenderCellsWithIntensity(width, height, levels, peaks, frame, 1)
}

func RenderCellsWithIntensity(width int, height int, levels []float64, peaks []float64, frame int, intensityScale float64) [][]Cell {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if width < 2 || height < 2 {
		return blankCells(width, height)
	}

	bands := width
	if len(levels) == 0 {
		levels = make([]float64, bands)
	}
	if len(peaks) == 0 {
		peaks = make([]float64, len(levels))
	}

	sampledLevels := resample(levels, bands)
	sampledPeaks := resample(peaks, bands)
	canvas := make([][]byte, height)
	tones := make([][]CellTone, height)
	for row := range canvas {
		canvas[row] = []byte(strings.Repeat(" ", width))
		tones[row] = make([]CellTone, width)
	}

	bass := average(sampledLevels[:maxInt(1, len(sampledLevels)/5)])
	energy := average(sampledLevels)
	centerX := float64(width-1) / 2
	centerY := float64(height-1) / 2
	shortSide := minInt(width, height)
	holeRadius := math.Max(3.0, float64(shortSide)*0.17+float64(shortSide)*0.02*bass)
	gravityRadius := holeRadius + math.Max(5, float64(shortSide)*0.44)
	intensityScale = clamp(intensityScale, 0, 1)

	drawMatrixRain(canvas, tones, sampledLevels, bass, energy, centerX, centerY, holeRadius, gravityRadius, frame, intensityScale)
	drawPeakSparks(canvas, sampledPeaks, centerX, centerY, holeRadius, frame)
	drawBlackHoleWithTones(canvas, tones, width, height, bass, energy, centerX, centerY, holeRadius, frame)

	return cellsFromCanvas(canvas, tones)
}

func drawMatrixRain(canvas [][]byte, tones [][]CellTone, levels []float64, bass, energy, centerX, centerY, holeRadius, gravityRadius float64, frame int, intensityScale float64) {
	height := len(canvas)
	width := len(canvas[0])
	intensityScale = clamp(intensityScale, 0, 1)
	if intensityScale <= 0 {
		return
	}
	for x, level := range levels {
		seed := hashInt(x*97 + 31)
		speed := 1 + int(level*4) + int(bass*2)
		trail := 2 + int(level*float64(height)*0.42) + int(bass*float64(height)*0.12)
		trail = minInt(height, maxInt(2, trail))
		head := positiveMod(frame*speed+seed, height)

		for y := 0; y < height; y++ {
			age := positiveMod(head-y, height)
			if age >= trail {
				continue
			}
			if matrixRainHiddenByStartupEase(x, y, frame, intensityScale) {
				continue
			}

			dx := float64(x) - centerX
			dy := float64(y) - centerY
			dist := math.Hypot(dx, dy)
			holePull := blackHoleInfluence(dx, dy, holeRadius)
			ringPull := blackHoleRingInfluence(dx, dy, holeRadius)

			pull := 0.0
			if dist < gravityRadius {
				pull = (gravityRadius - dist) / gravityRadius
			}
			bend := pull*pull + holePull*0.75 + ringPull*0.32
			swirl := int(bend*float64(width)*0.18 + bass*1.4)
			if dy < 0 {
				swirl = -swirl
			}
			if dx < 0 {
				swirl -= int(ringPull*float64(width)*0.04) + int(holePull*2)
			}
			if dx > 0 {
				swirl += int(ringPull*float64(width)*0.04) + int(holePull*2)
			}
			sink := int((pull*0.18 + holePull*0.32) * float64(height))
			warpedX := positiveMod(x+swirl, width)
			warpedY := positiveMod(y+sink+frame%3, height)

			fade := 1 - float64(age)/float64(trail)
			intensity := clamp(fade*0.60+level*0.34+bass*0.07+energy*0.12, 0, 1)
			if dist < holeRadius*0.95 {
				intensity *= 0.12
			} else if ringPull > 0.08 {
				intensity = clamp(intensity*0.52+ringPull*0.60+pull*0.18, 0, 1)
			} else if dist < holeRadius*1.75 {
				intensity = clamp(intensity*0.46+pull*0.24+holePull*0.34, 0, 1)
			} else if dist < gravityRadius {
				intensity = clamp(intensity+pull*0.22, 0, 1)
			}
			intensity *= intensityScale
			if intensity < 0.04 {
				continue
			}
			if intensity < 0.20 && (x+y+frame)%3 != 0 {
				continue
			}

			ch := matrixChar(warpedX, warpedY, frame, intensity)
			if ringPull > 0.18 && (x+y+frame)%2 == 0 {
				ch = auraGlyph(clamp(intensity+ringPull*0.32, 0, 1), x+y+frame)
			} else if dist < holeRadius*1.55 && (x+y+frame)%2 == 0 {
				ch = auraGlyph(intensity, x+y+frame)
			}
			canvas[y][x] = ch
			setTone(tones, x, y, matrixRainTone(intensity, level*intensityScale, energy*intensityScale, warpedX, warpedY, frame))
		}
	}
}

func matrixRainHiddenByStartupEase(x, y, frame int, intensityScale float64) bool {
	if intensityScale >= 0.98 {
		return false
	}
	visiblePercent := maxInt(1, int(intensityScale*100))
	roll := positiveMod(hashInt(x*131+y*29+(frame/6)*17), 100)
	return roll >= visiblePercent
}

func matrixRainTone(intensity, level, energy float64, x, y, frame int) CellTone {
	jitter := float64(positiveMod(hashInt(x*67+y*37+(frame/8)*19), 100)) / 100
	music := fibonacciMusicScale(level*0.70 + energy*0.30)
	score := clamp(intensity*0.52+music*0.43+(jitter-0.5)*0.22+0.03, 0, 1)
	switch {
	case score > 0.74:
		return ToneRainBright
	case score > 0.58:
		return ToneRainGlow
	case score > 0.42:
		return ToneRainSoft
	case score > 0.25:
		return ToneRainDim
	default:
		return ToneRainTrace
	}
}

func fibonacciMusicScale(value float64) float64 {
	steps := [...]float64{0, 1, 2, 3, 5, 8, 13}
	value = clamp(value, 0, 1)
	position := value * float64(len(steps)-1)
	lo := int(math.Floor(position))
	hi := minInt(len(steps)-1, lo+1)
	frac := position - float64(lo)
	return ((steps[lo] * (1 - frac)) + (steps[hi] * frac)) / steps[len(steps)-1]
}

func blackHoleInfluence(dx, dy, radius float64) float64 {
	dist := terminalDistance(dx, dy)
	return clamp(1-dist/(radius*2.6), 0, 1)
}

func blackHoleRingInfluence(dx, dy, radius float64) float64 {
	dist := terminalDistance(dx, dy)
	influence := 0.0
	for _, ringRadius := range blackHoleRingRadii(radius) {
		influence = math.Max(influence, clamp(1-math.Abs(dist-ringRadius)/(radius*0.34), 0, 1))
	}
	return influence
}

func terminalDistance(dx, dy float64) float64 {
	return math.Hypot(dx, dy*terminalCellAspect)
}

func drawBlackHole(canvas [][]byte, width, height int, bass, energy, centerX, centerY, radius float64, frame int) {
	drawBlackHoleWithTones(canvas, nil, width, height, bass, energy, centerX, centerY, radius, frame)
}

func drawBlackHoleWithTones(canvas [][]byte, tones [][]CellTone, width, height int, bass, energy, centerX, centerY, radius float64, frame int) {
	cx := int(centerX + 0.5)
	cy := int(centerY + 0.5)
	if width < 9 || height < 5 {
		setCell(canvas, cx, cy, ' ')
		setCell(canvas, cx-1, cy, '*')
		setCell(canvas, cx+1, cy, '*')
		setCell(canvas, cx, cy-1, '+')
		setCell(canvas, cx, cy+1, '+')
		return
	}

	coreRadius := radius * (1.12 + bass*0.05)
	ringRadii := blackHoleRingRadii(coreRadius)
	outerRingRadius := ringRadii[len(ringRadii)-1]
	reachX := int(math.Ceil(outerRingRadius + 1.3))
	reachY := int(math.Ceil((outerRingRadius + 1.3) / terminalCellAspect))
	minX := cx - reachX
	maxX := cx + reachX
	minY := cy - reachY
	maxY := cy + reachY
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if !inCanvas(canvas, x, y) {
				continue
			}
			dx := float64(x) - centerX
			dy := float64(y) - centerY
			dist := terminalDistance(dx, dy)

			if dist <= coreRadius {
				if canvas[y][x] != ' ' {
					setTone(tones, x, y, ToneDark)
				}
				continue
			}
			for index, ringRadius := range ringRadii {
				thickness := 0.58
				if index == 2 {
					thickness = 0.50
				}
				if math.Abs(dist-ringRadius) <= thickness {
					intensity := clamp(0.80-float64(index)*0.16+energy*0.12+bass*0.08, 0, 1)
					drawRainPermeableRingCell(canvas, x, y, blackHoleGlyph(intensity, x+y+frame+index*3))
					break
				}
			}
		}
	}
}

func setTone(tones [][]CellTone, x, y int, tone CellTone) {
	if y >= 0 && y < len(tones) && x >= 0 && x < len(tones[y]) {
		tones[y][x] = tone
	}
}

func blackHoleRingRadii(radius float64) []float64 {
	return []float64{
		radius * 1.34,
		radius * 1.72,
		radius * 2.10,
	}
}

func drawRainPermeableRingCell(canvas [][]byte, x, y int, glyph byte) {
	if canvas[y][x] != ' ' {
		return
	}
	canvas[y][x] = glyph
}

func setCell(canvas [][]byte, x, y int, glyph byte) {
	if inCanvas(canvas, x, y) {
		canvas[y][x] = glyph
	}
}

func inCanvas(canvas [][]byte, x, y int) bool {
	return y >= 0 && y < len(canvas) && x >= 0 && x < len(canvas[y])
}

func drawPeakSparks(canvas [][]byte, peaks []float64, centerX, centerY, holeRadius float64, frame int) {
	height := len(canvas)
	width := len(canvas[0])
	for x, peak := range resample(peaks, width) {
		if peak < 0.10 {
			continue
		}
		y := positiveMod(frame*(1+int(peak*4))+x*7, height)
		if math.Hypot(float64(x)-centerX, float64(y)-centerY) <= holeRadius*1.45 {
			y = positiveMod(y+int(holeRadius)+2, height)
		}
		canvas[y][x] = sparkGlyph(peak, x+frame)
	}
}

func matrixChar(x, y, frame int, intensity float64) byte {
	if intensity > 0.82 {
		if (x+y+frame)%5 == 0 {
			return '#'
		}
		return matrixGlyphs[positiveMod(hashInt(x*41+y*17+frame*7), len(matrixGlyphs))]
	}
	if intensity > 0.58 {
		return matrixGlyphs[positiveMod(hashInt(x*31+y*19+frame*5), len(matrixGlyphs))]
	}
	if intensity > 0.36 {
		return fieldGlyphs[6+positiveMod(x+y+frame, 3)]
	}
	return fieldGlyphs[2+positiveMod(x*3+y+frame, 3)]
}

func auraGlyph(intensity float64, position int) byte {
	switch {
	case intensity > 0.88:
		if position%2 == 0 {
			return '#'
		}
		return '@'
	case intensity > 0.62:
		return '*'
	case intensity > 0.36:
		return '+'
	default:
		return '-'
	}
}

func blackHoleGlyph(intensity float64, position int) byte {
	switch {
	case intensity > 0.82:
		if position%2 == 0 {
			return '#'
		}
		return '@'
	case intensity > 0.58:
		return '*'
	case intensity > 0.32:
		return '='
	default:
		return '-'
	}
}

func sparkGlyph(peak float64, position int) byte {
	if peak > 0.82 {
		if position%2 == 0 {
			return '*'
		}
		return '@'
	}
	if peak > 0.48 {
		return '+'
	}
	return '\''
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func hashInt(value int) int {
	value ^= value << 13
	value ^= value >> 17
	value ^= value << 5
	if value < 0 {
		return -value
	}
	return value
}

func positiveMod(value, modulo int) int {
	if modulo <= 0 {
		return 0
	}
	value %= modulo
	if value < 0 {
		value += modulo
	}
	return value
}

func resample(levels []float64, bands int) []float64 {
	if bands <= 0 {
		return nil
	}
	if len(levels) == 0 {
		return make([]float64, bands)
	}
	if len(levels) == bands {
		out := make([]float64, bands)
		for i, level := range levels {
			out[i] = clamp(level, 0, 1)
		}
		return out
	}
	if len(levels) == 1 {
		out := make([]float64, bands)
		value := clamp(levels[0], 0, 1)
		for i := range out {
			out[i] = value
		}
		return out
	}

	out := make([]float64, 0, bands)
	srcMax := len(levels) - 1
	for i := 0; i < bands; i++ {
		position := (float64(i) * float64(srcMax)) / float64(maxInt(1, bands-1))
		lo := int(position)
		hi := minInt(srcMax, lo+1)
		frac := position - float64(lo)
		value := levels[lo]*(1-frac) + levels[hi]*frac
		out = append(out, clamp(value, 0, 1))
	}
	return out
}

func blankCells(width int, height int) [][]Cell {
	cells := make([][]Cell, height)
	for y := range cells {
		cells[y] = make([]Cell, width)
		for x := range cells[y] {
			cells[y][x] = Cell{Glyph: ' '}
		}
	}
	return cells
}

func cellsFromCanvas(canvas [][]byte, tones [][]CellTone) [][]Cell {
	cells := make([][]Cell, len(canvas))
	for y := range canvas {
		cells[y] = make([]Cell, len(canvas[y]))
		for x, glyph := range canvas[y] {
			cells[y][x] = Cell{Glyph: glyph}
			if y < len(tones) && x < len(tones[y]) {
				cells[y][x].Tone = tones[y][x]
			}
		}
	}
	return cells
}

func cellLines(cells [][]Cell) []string {
	lines := make([]string, 0, len(cells))
	for _, row := range cells {
		line := make([]byte, len(row))
		for x, cell := range row {
			line[x] = cell.Glyph
		}
		lines = append(lines, string(line))
	}
	return lines
}
