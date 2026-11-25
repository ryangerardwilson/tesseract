package download

import (
	"regexp"
	"strconv"
)

var (
	percentPattern = regexp.MustCompile(`\[download\]\s+([0-9]+(?:\.[0-9]+)?)%`)
	speedPattern   = regexp.MustCompile(`\bat\s+(\S+)`)
	etaPattern     = regexp.MustCompile(`\bETA\s+(\S+)`)
)

// ParseProgress reads one yt-dlp progress line.
func ParseProgress(line string) (percent float64, speed, eta string, ok bool) {
	match := percentPattern.FindStringSubmatch(line)
	if match == nil {
		return 0, "", "", false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, "", "", false
	}
	if found := speedPattern.FindStringSubmatch(line); found != nil {
		speed = found[1]
	}
	if found := etaPattern.FindStringSubmatch(line); found != nil {
		eta = found[1]
	}
	return value, speed, eta, true
}
