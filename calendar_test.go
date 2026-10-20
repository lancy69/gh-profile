package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func yearCalendar() contributionCalendar {
	start := time.Date(2024, 2, 25, 0, 0, 0, 0, time.UTC)
	c := contributionCalendar{StartedAt: start.AddDate(0, 0, 3), EndedAt: start.AddDate(0, 0, 366), Total: 1234}
	for i := range 53 {
		first := start.AddDate(0, 0, 7*i)
		w := contributionWeek{FirstDay: first.Format(time.DateOnly)}
		for weekday := range 7 {
			day := first.AddDate(0, 0, weekday)
			if day.Before(c.StartedAt) || day.After(c.EndedAt) {
				continue
			}
			w.Days = append(w.Days, contributionDay{day.Format(time.DateOnly), weekday, i % 5, contributionLevels[(i+weekday)%5]})
		}
		c.Weeks = append(c.Weeks, w)
	}
	return c
}
func TestPartialLeapCalendarPlacement(t *testing.T) {
	v := fixtureViewer()
	data, err := jsonViewer(v)
	if err != nil {
		t.Fatal(err)
	}
	c, err := normalizeCalendar(data)
	if err != nil {
		t.Fatal(err)
	}
	grid := renderCalendarBlock(c.Weeks, 28, false)
	lines := strings.Split(grid, "\n")
	expected := map[string]string{"Sun ": "    ", "Mon ": "    ", "Tue ": "  . ", "Wed ": ".   ", "Thu ": "4   ", "Fri ": "    ", "Sat ": "    "}
	for _, line := range lines {
		if want, ok := expected[line[:min(4, len(line))]]; ok {
			if line[4:] != want {
				t.Fatalf("wrong boundary/day placement %q want %q", line, want)
			}
			delete(expected, line[:4])
		}
	}
	if len(expected) != 0 {
		t.Fatalf("missing weekdays: %v", expected)
	}
}
func TestCalendarPreservesWeeksAndLevels(t *testing.T) {
	c := yearCalendar()
	c.Weeks[20].Days[3].Level = "FUTURE_LEVEL"
	for _, width := range []int{28, 78, 110, 142} {
		for _, color := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%v", width, color), func(t *testing.T) {
				s := renderCalendar(c, width, color)
				assertBounded(t, s, width)
				got := ansi.Strip(s)
				rows := 0
				cells := 0
				for _, line := range strings.Split(got, "\n") {
					isRow := false
					for _, name := range []string{"Sun ", "Mon ", "Tue ", "Wed ", "Thu ", "Fri ", "Sat "} {
						if strings.HasPrefix(line, name) {
							isRow = true
						}
					}
					if !isRow {
						continue
					}
					rows++
					for _, r := range line[4:] {
						if strings.ContainsRune(".1234?·■", r) {
							cells++
						}
					}
				}
				days := 0
				for _, w := range c.Weeks {
					days += len(w.Days)
				}
				if cells != days {
					t.Fatalf("lost/duplicated dates: got %d want %d", cells, days)
				}
				blocks := (53 + max(1, (width-4)/2) - 1) / max(1, (width-4)/2)
				if rows != blocks*7 {
					t.Fatalf("lost blocks: %d rows", rows)
				}
				if !strings.Contains(strings.Join(strings.Fields(got), ""), "?unknownlevel") || !strings.Contains(got, c.Weeks[0].Days[0].Date) || !strings.Contains(got, c.Weeks[52].Days[len(c.Weeks[52].Days)-1].Date) {
					t.Fatal("missing edge date or unknown legend")
				}
				if !color {
					for _, symbol := range []string{". ", "1 ", "2 ", "3 ", "4 ", "? "} {
						if !strings.Contains(got, symbol) {
							t.Fatalf("missing level %q", symbol)
						}
					}
				}
			})
		}
	}
}
func TestCalendarZeroAndEmpty(t *testing.T) {
	c := yearCalendar()
	c.Total = 0
	for i := range c.Weeks {
		for j := range c.Weeks[i].Days {
			c.Weeks[i].Days[j].Level = "NONE"
			c.Weeks[i].Days[j].Count = 0
		}
	}
	got := renderCalendar(c, 78, false)
	if !strings.Contains(got, "Sun ") || !strings.Contains(got, ". ") || strings.Contains(got, "No contribution data") {
		t.Fatal("zero year lost grid")
	}
	c.Weeks = nil
	if !strings.Contains(renderCalendar(c, 28, false), "No contribution data") {
		t.Fatal("missing empty-calendar state")
	}
}
