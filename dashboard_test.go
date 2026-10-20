package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func testDashboard(t *testing.T, loader profileLoader) *dashboard {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return newDashboard(ctx, cancel, loader, false)
}
func keyMsg(code rune) tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Code: code}) }
func TestResizeAndScrollLifecycle(t *testing.T) {
	calls := 0
	m := testDashboard(t, func(context.Context) (profile, error) { calls++; return richProfile(), nil })
	command := m.Init()
	if calls != 0 {
		t.Fatal("loader ran synchronously")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(m.viewport.GetContent(), "Loading") {
		t.Fatal("no loading state")
	}
	m.Update(command())
	if calls != 1 || m.status != statusLoaded {
		t.Fatal("load failed")
	}
	content := m.viewport.GetContent()
	m.Update(keyMsg('j'))
	if m.viewport.YOffset() != 1 {
		t.Fatal("j did not scroll")
	}
	m.Update(keyMsg(tea.KeyPgDown))
	if m.viewport.YOffset() <= 1 {
		t.Fatal("page did not scroll")
	}
	m.Update(keyMsg(tea.KeyEnd))
	if !m.viewport.AtBottom() || !strings.Contains(m.viewport.View(), "More") {
		t.Fatal("calendar end unreachable")
	}
	oldOffset := m.viewport.YOffset()
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	if m.viewport.YOffset() != oldOffset || m.viewport.GetContent() != content {
		t.Fatal("small size discarded content/scroll")
	}
	if !strings.Contains(m.View().Content, "Resize") {
		t.Fatal("missing resize notice")
	}
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	if calls != 1 || m.viewport.PastBottom() || m.viewport.YOffset() > oldOffset {
		t.Fatal("resize refetched or failed to clamp")
	}
	m.Update(keyMsg(tea.KeyHome))
	if !m.viewport.AtTop() || !strings.Contains(m.viewport.View(), "@tester") {
		t.Fatal("identity unreachable")
	}
	m.Update(keyMsg('j'))
	m.Update(keyMsg('k'))
	if !m.viewport.AtTop() {
		t.Fatal("k did not scroll back")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if calls != 1 {
		t.Fatal("resize refetched")
	}
}
func TestLoadBeforeSizeAndFailure(t *testing.T) {
	m := testDashboard(t, func(context.Context) (profile, error) { return profile{}, nil })
	m.Update(profileResultMsg{profile: richProfile()})
	if m.viewport.GetContent() != "" {
		t.Fatal("rendered with unknown size")
	}
	m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	assertBounded(t, m.viewport.GetContent(), 28)
	loadErr := errors.New("transport EOF \x1b]52;c;bad\a\u202eevil")
	m.Update(profileResultMsg{err: loadErr})
	if m.loadErr != loadErr || m.status != statusFailed || !strings.Contains(m.viewport.GetContent(), "transport EOF") || strings.ContainsAny(m.viewport.GetContent(), "\x1b\u202e") {
		t.Fatal("failed cause missing or unsafe")
	}
	m.Update(keyMsg('q'))
	if m.loadErr != loadErr {
		t.Fatal("failed load lost on quit")
	}
}
func TestQuitCancelsLoader(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{keyMsg('q'), tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})} {
		started := make(chan struct{})
		finished := make(chan tea.Msg, 1)
		m := testDashboard(t, func(ctx context.Context) (profile, error) { close(started); <-ctx.Done(); return profile{}, ctx.Err() })
		cmd := m.Init()
		go func() { finished <- cmd() }()
		<-started
		_, quit := m.Update(key)
		if quit == nil {
			t.Fatal("quit command missing")
		}
		select {
		case msg := <-finished:
			m.Update(msg)
		case <-time.After(time.Second):
			t.Fatal("loader not canceled")
		}
		if m.loadErr != nil || m.ctx.Err() != context.Canceled {
			t.Fatal("loading cancellation became failure")
		}
	}
}
func TestTinySizesRemainBounded(t *testing.T) {
	m := testDashboard(t, nil)
	for _, size := range []tea.WindowSizeMsg{{Width: 0, Height: 0}, {Width: 1, Height: 1}, {Width: 29, Height: 9}, {Width: 80, Height: 1}} {
		m.Update(size)
		v := m.View()
		assertBounded(t, v.Content, size.Width)
		if v.Content != "" && len(strings.Split(v.Content, "\n")) > size.Height {
			t.Fatal("tiny view exceeds height")
		}
	}
}
func TestRejectNonTerminalBeforeLoad(t *testing.T) {
	in, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	called := false
	err = run(context.Background(), in, out, func(context.Context) (profile, error) { called = true; return profile{}, nil })
	if err == nil || err.Error() != "an interactive terminal is required" || called {
		t.Fatalf("terminal guard failed: called=%v err=%v", called, err)
	}
	info, _ := out.Stat()
	if info.Size() != 0 {
		t.Fatal("terminal escapes emitted before guard")
	}
}
