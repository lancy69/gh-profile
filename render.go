package main

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Remote text is never allowed to introduce terminal controls or bidi overrides.
func sanitizeText(s string) string {
	s = ansi.Strip(strings.ToValidUTF8(s, "�"))
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func wrapText(s string, width int) string {
	if width < 1 {
		return ""
	}
	return ansi.Wrap(s, width, "")
}
func heading(s string, color bool) string {
	if !color {
		return s
	}
	return lipgloss.NewStyle().Bold(true).Render(s)
}
func accent(s string, color bool) string {
	if !color {
		return s
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#58a6ff")).Render(s)
}
func renderIdentity(p profile, width int, color bool) string {
	name := sanitizeText(p.Name)
	if name == "" {
		name = sanitizeText(p.Login)
	}
	fields := []string{heading(wrapText(name, width), color), accent(wrapText("@"+sanitizeText(p.Login), width), color)}
	if bio := sanitizeText(p.Bio); bio != "" {
		fields = append(fields, "", wrapText(bio, width))
	}
	fields = append(fields, "", wrapText(fmt.Sprintf("%d followers · %d following", p.Followers, p.Following), width))
	for _, f := range []struct{ label, value string }{{"Company: ", p.Company}, {"Location: ", p.Location}, {"Email: ", p.Email}} {
		if value := sanitizeText(f.value); value != "" {
			fields = append(fields, wrapText(f.label+value, width))
		}
	}
	if value := sanitizeText(p.WebsiteURL); value != "" {
		fields = append(fields, accent(wrapText(value, width), color))
	}
	for _, s := range p.Socials {
		label := sanitizeText(s.DisplayName)
		provider := sanitizeText(s.Provider)
		if label == "" {
			label = provider
		} else if provider != "" && !strings.EqualFold(label, provider) {
			label += " (" + provider + ")"
		}
		if label != "" {
			label += ": "
		}
		fields = append(fields, accent(wrapText(label+sanitizeText(s.URL), width), color))
	}
	if value := sanitizeText(p.URL); value != "" {
		fields = append(fields, accent(wrapText(value, width), color))
	}
	return strings.Join(fields, "\n")
}
func cardContent(p pinnedRepository, width int, color bool) string {
	visibility := "Public"
	if p.Private {
		visibility = "Private"
	}
	fields := []string{accent(wrapText(sanitizeText(p.NameWithOwner), width), color), visibility}
	if d := sanitizeText(p.Description); d != "" {
		fields = append(fields, wrapText(d, width))
	}
	fields = append(fields, accent(wrapText(sanitizeText(p.URL), width), color))
	if l := sanitizeText(p.Language); l != "" {
		fields = append(fields, wrapText(l, width))
	}
	fields = append(fields, wrapText(fmt.Sprintf("Stars: %d · Forks: %d", p.Stars, p.Forks), width))
	return strings.Join(fields, "\n")
}
func cardBox(content string, width, height int, color bool) string {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(width).Height(height + 2)
	if color {
		style = style.BorderForeground(lipgloss.Color("#6e7681"))
	}
	return style.Render(content)
}
func renderPins(pins []pinnedRepository, width int, color bool) string {
	lines := []string{heading(wrapText("Pinned repositories", width), color)}
	if len(pins) == 0 {
		return strings.Join(append(lines, wrapText("No pinned repositories.", width)), "\n")
	}
	columns := 1
	if width >= 82 {
		columns = 2
	}
	for i := 0; i < len(pins); i += columns {
		firstWidth := width
		if columns == 2 {
			firstWidth = (width - 2) / 2
		}
		a := cardContent(pins[i], max(1, firstWidth-4), color)
		h := strings.Count(a, "\n") + 1
		if columns == 2 && i+1 < len(pins) {
			secondWidth := width - 2 - firstWidth
			b := cardContent(pins[i+1], secondWidth-4, color)
			h = max(h, strings.Count(b, "\n")+1)
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, cardBox(a, firstWidth, h, color), "  ", cardBox(b, secondWidth, h, color)))
		} else {
			lines = append(lines, cardBox(a, firstWidth, h, color))
		}
	}
	return strings.Join(lines, "\n")
}
func renderProfile(p profile, width int, color bool) string {
	if width < 4 {
		return ""
	}
	if width >= 142 {
		identity := lipgloss.NewStyle().Width(26).Render(renderIdentity(p, 26, color))
		rightWidth := width - 29
		right := renderPins(p.Pins, rightWidth, color) + "\n\n" + renderCalendar(p.Calendar, rightWidth, color)
		return lipgloss.JoinHorizontal(lipgloss.Top, identity, "   ", right)
	}
	return renderIdentity(p, width, color) + "\n\n" + renderPins(p.Pins, width, color) + "\n\n" + renderCalendar(p.Calendar, width, color)
}
