package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func richProfile() profile {
	p := profile{Login: "tester", Name: strings.Repeat("名前👩‍💻", 12), Bio: "Biography\nwith multiline\tdescription", Company: "Company", Location: "Earth", Email: "public@example.com", WebsiteURL: "https://example.com/" + strings.Repeat("longpath", 24), URL: "https://github.com/tester", Followers: 1234, Following: 12, Socials: []socialLink{{"Someone", "MASTODON", "https://social.example/@someone"}}, Calendar: yearCalendar()}
	for i := range 6 {
		p.Pins = append(p.Pins, pinnedRepository{NameWithOwner: fmt.Sprintf("other/project%d", i), Description: strings.Repeat("CJK名字 👩‍💻 a-long-word-without-spaces ", i+1), URL: "https://github.com/" + strings.Repeat("path", 25), Private: i%2 == 1, Language: "Go", Stars: 1000 + i, Forks: i})
	}
	return p
}
func assertBounded(t *testing.T, s string, width int) {
	t.Helper()
	for i, line := range strings.Split(s, "\n") {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("line %d: width %d > %d: %q", i, got, width, line)
		}
	}
}
func TestSanitizeRemoteText(t *testing.T) {
	input := "safe\x1b]52;c;YXR0YWNr\a\x1b[31m red\x1b[0m\x00\x7f\u0080\u202e\u2066\r\n\t漢字👩‍💻\xff"
	got := sanitizeText(input)
	if got != "safe red 漢字👩‍💻�" {
		t.Fatalf("unsafe or damaged text: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatal("invalid UTF-8")
	}
	p := richProfile()
	p.Name = input
	p.Pins[0].Description = input
	p.Socials[0].URL = input
	s := renderProfile(p, 78, false)
	if strings.ContainsAny(s, "\x1b\x00\x7f\u202e\u2066") || strings.Contains(s, "YXR0YWNr") {
		t.Fatal("remote terminal controls survived")
	}
}
func TestProfileWidthsAndContent(t *testing.T) {
	p := richProfile()
	for _, width := range []int{28, 29, 78, 81, 82, 83, 112, 141, 142, 143, 158} {
		for _, color := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%v", width, color), func(t *testing.T) {
				s := renderProfile(p, width, color)
				assertBounded(t, s, width)
				text := ansi.Strip(s)
				// Requested content is wrapped, never truncated; remove layout whitespace for URLs.
				compact := strings.Join(strings.Fields(text), "")
				for _, want := range []string{"public@example.com", "Public", "Private", "Stars:1000", "other/project5", "MASTODON"} {
					if !strings.Contains(compact, strings.Join(strings.Fields(want), "")) {
						t.Fatalf("missing %q", want)
					}
				}
				identityWidth := width
				if width >= 142 {
					identityWidth = 26
				}
				identity := strings.Join(strings.Fields(ansi.Strip(renderIdentity(p, identityWidth, color))), "")
				if !strings.Contains(identity, "1234followers·12following") || !strings.Contains(identity, p.WebsiteURL) {
					t.Fatal("identity content truncated")
				}
			})
		}
	}
}
func TestEmptyProfileRendering(t *testing.T) {
	s := renderProfile(profile{Login: "fallback"}, 78, false)
	if !strings.HasPrefix(s, "fallback\n@fallback") || !strings.Contains(s, "No pinned repositories.") || strings.Contains(s, "Email:") || strings.Contains(s, "Company:") {
		t.Fatalf("wrong empty state: %s", s)
	}
}
func TestCardThresholdAndOrder(t *testing.T) {
	pins := []pinnedRepository{{NameWithOwner: "a/first", Description: strings.Repeat("description ", 20)}, {NameWithOwner: "b/second"}, {NameWithOwner: "c/third"}}
	narrow := renderPins(pins, 81, false)
	wide := renderPins(pins, 82, false)
	assertBounded(t, narrow, 81)
	assertBounded(t, wide, 82)
	firstLine := -1
	secondLine := -1
	for i, l := range strings.Split(wide, "\n") {
		if strings.Contains(l, "a/first") {
			firstLine = i
		}
		if strings.Contains(l, "b/second") {
			secondLine = i
		}
	}
	if firstLine != secondLine || firstLine < 0 {
		t.Fatal("two columns must use row-major order")
	}
	if strings.Index(narrow, "a/first") > strings.Index(narrow, "b/second") || strings.Index(wide, "b/second") > strings.Index(wide, "c/third") {
		t.Fatal("pin order changed")
	}
}
func TestCardRowHasEqualHeightAndFullWidth(t *testing.T) {
	pins := []pinnedRepository{{NameWithOwner: "a/short"}, {NameWithOwner: "b/long", URL: "https://example.com/" + strings.Repeat("unbrokenpath", 25)}}
	rendered := renderPins(pins, 83, false)
	bottoms := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "╰") {
			bottoms++
			if strings.Count(line, "╰") != 2 || ansi.StringWidth(line) != 83 {
				t.Fatalf("unequal or undersized card row: %q", line)
			}
		}
	}
	if bottoms != 1 {
		t.Fatalf("card row has %d bottom lines", bottoms)
	}
}
