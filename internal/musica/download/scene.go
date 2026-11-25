package download

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryangerardwilson/tesseract/internal/musica/store"
)

var (
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	textStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
)

var (
	leftReel  = []string{"◜", "◝", "◞", "◟"}
	rightReel = []string{"◟", "◞", "◝", "◜"}
)

// Render draws the download list. A running job changes with frame.
func Render(jobs []store.Job, selected, frame, width int, playing string) string {
	if width < 20 {
		width = 20
	}
	lines := []string{titleStyle.Render("downloads")}
	if playing != "" {
		lines = append(lines, textStyle.Render("playing  "+fit(playing, width-10)))
	}
	if len(jobs) == 0 {
		lines = append(lines, "", textStyle.Render("no downloads"), "", textStyle.Render("y download into the open cassette"))
		return strings.Join(lines, "\n")
	}
	for i, job := range jobs {
		lines = append(lines, "", renderJob(job, i == selected, frame, width))
	}
	lines = append(lines, "", textStyle.Render("x cancel    esc library    y another"))
	return strings.Join(lines, "\n")
}

func renderJob(job store.Job, selected bool, frame, width int) string {
	mark := "  "
	if selected {
		mark = "> "
	}
	left, right := "○", "○"
	if job.Status == "running" {
		left = leftReel[frame%len(leftReel)]
		right = rightReel[frame%len(rightReel)]
	}
	barWidth := width - 28
	if barWidth < 8 {
		barWidth = 8
	}
	if barWidth > 32 {
		barWidth = 32
	}
	bar := progressBar(shownPercent(job), barWidth)
	headline := fmt.Sprintf("%s%s %s %s  %3.0f%%  %s", mark, left, bar, right, shownPercent(job), fit(job.Title, 28))
	detail := mark + "  " + jobDetail(job)
	return lineStyle(job.Status, selected).Render(headline) + "\n" + lineStyle(job.Status, false).Render(detail)
}

func lineStyle(_ string, selected bool) lipgloss.Style {
	if selected {
		return titleStyle
	}
	return textStyle
}

func shownPercent(job store.Job) float64 {
	switch job.Status {
	case "done":
		return 100
	case "queued", "cancelled":
		return 0
	default:
		if job.Percent < 0 {
			return 0
		}
		if job.Percent > 100 {
			return 100
		}
		return job.Percent
	}
}

func progressBar(percent float64, width int) string {
	if width < 1 {
		width = 1
	}
	fill := int(percent / 100 * float64(width))
	if fill < 0 {
		fill = 0
	}
	if fill > width {
		fill = width
	}
	return strings.Repeat("█", fill) + strings.Repeat("░", width-fill)
}

func jobDetail(job store.Job) string {
	switch job.Status {
	case "running":
		speed := job.Speed
		if speed == "" {
			speed = "starting"
		}
		eta := job.ETA
		if eta == "" {
			eta = "…"
		}
		return speed + "    ETA " + eta
	case "queued":
		return "waiting"
	case "done":
		return "in the cassette"
	case "cancelled":
		return "cancelled"
	case "failed":
		if job.Error == "" {
			return "failed"
		}
		return "failed  " + fit(job.Error, 48)
	default:
		return job.Status
	}
}

func fit(value string, width int) string {
	if width < 1 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}
