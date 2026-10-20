package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type loadStatus uint8

const (
	statusLoading loadStatus = iota
	statusLoaded
	statusFailed
)

type profileResultMsg struct {
	profile profile
	err     error
}
type dashboard struct {
	viewport      viewport.Model
	width, height int
	sizeKnown     bool
	status        loadStatus
	profile       profile
	loadErr       error
	ctx           context.Context
	cancel        context.CancelFunc
	loader        profileLoader
	color         bool
}

func newDashboard(ctx context.Context, cancel context.CancelFunc, load profileLoader, color bool) *dashboard {
	v := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	v.SoftWrap = false
	v.FillHeight = true
	v.MouseWheelEnabled = false
	v.SetHorizontalStep(0)
	return &dashboard{viewport: v, ctx: ctx, cancel: cancel, loader: load, color: color}
}
func run(ctx context.Context, in, out *os.File, load profileLoader) error {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) || os.Getenv("TERM") == "dumb" {
		return errors.New("an interactive terminal is required")
	}
	programCtx := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	model := newDashboard(ctx, cancel, load, os.Getenv("NO_COLOR") == "")
	result, err := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out), tea.WithContext(programCtx)).Run()
	if err != nil {
		return fmt.Errorf("run dashboard: %w", err)
	}
	if final, ok := result.(*dashboard); ok {
		return final.loadErr
	}
	return nil
}
func (m *dashboard) Init() tea.Cmd {
	return func() tea.Msg { p, err := m.loader(m.ctx); return profileResultMsg{p, err} }
}
func (m *dashboard) supportedSize() bool { return m.sizeKnown && m.width >= 30 && m.height >= 10 }
func (m *dashboard) render() {
	if !m.supportedSize() {
		return
	}
	offset := m.viewport.YOffset()
	m.viewport.SetWidth(m.width - 2)
	m.viewport.SetHeight(m.height - 1)
	var content string
	switch m.status {
	case statusLoading:
		content = "Loading profile…"
	case statusLoaded:
		content = renderProfile(m.profile, m.width-2, m.color)
	case statusFailed:
		content = heading("Unable to load profile", m.color) + "\n\n" + wrapText(sanitizeText(m.loadErr.Error()), m.width-2) + "\n\n" + wrapText("Check gh auth status / gh auth login if authentication is required. Press q to quit.", m.width-2)
	}
	m.viewport.SetContent(content)
	m.viewport.SetYOffset(offset)
}
func (m *dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "home":
			if m.supportedSize() {
				m.viewport.GotoTop()
			}
			return m, nil
		case "end":
			if m.supportedSize() {
				m.viewport.GotoBottom()
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = max(0, msg.Width)
		m.height = max(0, msg.Height)
		m.sizeKnown = true
		m.render()
		return m, nil
	case profileResultMsg:
		if m.ctx.Err() != nil {
			return m, nil
		}
		if msg.err != nil {
			m.status = statusFailed
			m.loadErr = msg.err
		} else {
			m.status = statusLoaded
			m.profile = msg.profile
		}
		m.render()
		return m, nil
	}
	if m.supportedSize() {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}
func (m *dashboard) View() tea.View {
	content := ""
	if m.sizeKnown {
		if !m.supportedSize() {
			if m.width > 0 && m.height > 0 {
				content = ansi.Truncate("Resize terminal (min 30×10)", m.width, "")
			}
		} else {
			footer := "↑/↓ scroll · PgUp/PgDn page · q quit"
			if ansi.StringWidth(footer) > m.width-2 {
				footer = "q quit"
			}
			// Viewport content is cached; keypresses only select its visible lines.
			content = lipgloss.NewStyle().Padding(0, 1).Render(m.viewport.View()) + "\n " + ansi.Truncate(footer, m.width-2, "")
		}
	}
	v := tea.NewView(strings.TrimSuffix(content, "\n"))
	v.AltScreen = true
	return v
}
