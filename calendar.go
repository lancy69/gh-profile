package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var contributionLevels = [...]string{"NONE", "FIRST_QUARTILE", "SECOND_QUARTILE", "THIRD_QUARTILE", "FOURTH_QUARTILE"}
var contributionColors = [...]string{"#6e7681", "#0e4429", "#006d32", "#26a641", "#39d353"}

func contributionCell(level string, color bool) string {
	for i, name := range contributionLevels {
		if level != name {
			continue
		}
		if !color {
			if i == 0 {
				return ". "
			}
			return fmt.Sprintf("%d ", i)
		}
		symbol := "■ "
		if i == 0 {
			symbol = "· "
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(contributionColors[i])).Render(symbol)
	}
	if color {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(contributionColors[0])).Render("? ")
	}
	return "? "
}
func blockDateRange(weeks []contributionWeek) (string, string) {
	first, last := "", ""
	for _, w := range weeks {
		for _, d := range w.Days {
			if first == "" || d.Date < first {
				first = d.Date
			}
			if d.Date > last {
				last = d.Date
			}
		}
	}
	if first == "" {
		first = weeks[0].FirstDay
		last = weeks[len(weeks)-1].FirstDay
	}
	return first, last
}
func renderCalendarBlock(weeks []contributionWeek, width int, color bool) string {
	first, last := blockDateRange(weeks)
	lines := []string{wrapText(first+" – "+last, width)}
	// Labels use the actual week dates, including month transitions inside a week.
	var labels strings.Builder
	labels.WriteString("    ")
	occupied := 4
	previousMonth := time.Month(0)
	for i, w := range weeks {
		start, _ := time.Parse(time.DateOnly, w.FirstDay)
		month := start.Month()
		if i > 0 {
			for _, d := range w.Days {
				date, _ := time.Parse(time.DateOnly, d.Date)
				if date.Day() == 1 {
					month = date.Month()
					break
				}
			}
		}
		candidate := i == 0 || month != previousMonth
		previousMonth = month
		if !candidate {
			continue
		}
		label := month.String()[:3]
		position := 4 + 2*i
		if position < occupied || position+3 > min(width, 4+len(weeks)*2) {
			continue
		}
		labels.WriteString(strings.Repeat(" ", position-occupied))
		labels.WriteString(label)
		occupied = position + 3
	}
	lines = append(lines, labels.String())
	for weekday, name := range []string{"Sun ", "Mon ", "Tue ", "Wed ", "Thu ", "Fri ", "Sat "} {
		var line strings.Builder
		line.WriteString(name)
		for _, w := range weeks {
			cell := "  "
			for _, d := range w.Days {
				if d.Weekday == weekday {
					cell = contributionCell(d.Level, color)
					break
				}
			}
			line.WriteString(cell)
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}
func renderCalendar(c contributionCalendar, width int, color bool) string {
	if width < 6 {
		return ""
	}
	parts := []string{heading(wrapText(fmt.Sprintf("%d contributions in the last year", c.Total), width), color), wrapText(c.StartedAt.Format(time.DateOnly)+" – "+c.EndedAt.Format(time.DateOnly), width), wrapText("Counts reflect your GitHub visibility and access.", width)}
	if len(c.Weeks) == 0 {
		return strings.Join(append(parts, wrapText("No contribution data available.", width)), "\n")
	}
	perBlock := max(1, (width-4)/2)
	unknown := false
	for _, w := range c.Weeks {
		for _, d := range w.Days {
			known := false
			for _, level := range contributionLevels {
				if d.Level == level {
					known = true
					break
				}
			}
			if !known {
				unknown = true
			}
		}
	}
	for start := 0; start < len(c.Weeks); start += perBlock {
		parts = append(parts, "", renderCalendarBlock(c.Weeks[start:min(start+perBlock, len(c.Weeks))], width, color))
	}
	legend := "Less "
	for _, level := range contributionLevels {
		legend += contributionCell(level, color)
	}
	legend += "More"
	if unknown {
		legend += " · ? unknown level"
	}
	parts = append(parts, "", ansi.Wrap(legend, width, ""))
	return strings.Join(parts, "\n")
}
