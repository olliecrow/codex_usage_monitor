package tui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/olliecrow/codex_usage_monitor/internal/usage"
)

type FetchFunc func(context.Context) (*usage.Summary, error)

type Options struct {
	Interval  time.Duration
	Timeout   time.Duration
	NoColor   bool
	AltScreen bool
	Fetch     FetchFunc
}

type Model struct {
	interval time.Duration
	timeout  time.Duration
	fetch    FetchFunc

	width  int
	height int

	now time.Time

	fetching          bool
	lastAttemptAt     time.Time
	lastSuccessAt     time.Time
	lastFetchDuration time.Duration
	lastError         string
	nextFetchAt       time.Time

	summary *usage.Summary
	styles  styles
}

type styles struct {
	title   lipgloss.Style
	dim     lipgloss.Style
	panel   lipgloss.Style
	label   lipgloss.Style
	value   lipgloss.Style
	ok      lipgloss.Style
	warn    lipgloss.Style
	bad     lipgloss.Style
	accent  lipgloss.Style
	error   lipgloss.Style
	help    lipgloss.Style
	mono    lipgloss.Style
	loading lipgloss.Style
}

type pollTickMsg struct {
	at time.Time
}

type clockTickMsg struct {
	at time.Time
}

type fetchResultMsg struct {
	at       time.Time
	duration time.Duration
	summary  *usage.Summary
	err      error
}

const (
	defaultInterval = 60 * time.Second
	defaultTimeout  = 10 * time.Second
)

func NewModel(opts Options) Model {
	interval := opts.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	fetch := opts.Fetch
	if fetch == nil {
		fetch = func(context.Context) (*usage.Summary, error) {
			return nil, errors.New("missing fetch function")
		}
	}
	now := time.Now().UTC()

	return Model{
		interval:    interval,
		timeout:     timeout,
		fetch:       fetch,
		now:         now,
		fetching:    true,
		nextFetchAt: now.Add(interval),
		styles:      defaultStyles(opts.NoColor),
	}
}

func defaultStyles(noColor bool) styles {
	basePanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if noColor {
		return styles{
			title:   lipgloss.NewStyle().Bold(true),
			dim:     lipgloss.NewStyle(),
			panel:   basePanel,
			label:   lipgloss.NewStyle().Bold(true),
			value:   lipgloss.NewStyle(),
			ok:      lipgloss.NewStyle().Bold(true),
			warn:    lipgloss.NewStyle().Bold(true),
			bad:     lipgloss.NewStyle().Bold(true),
			accent:  lipgloss.NewStyle().Bold(true),
			error:   lipgloss.NewStyle().Bold(true),
			help:    lipgloss.NewStyle(),
			mono:    lipgloss.NewStyle(),
			loading: lipgloss.NewStyle(),
		}
	}
	return styles{
		title:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Padding(0, 1),
		dim:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		panel:   basePanel.BorderForeground(lipgloss.Color("61")),
		label:   lipgloss.NewStyle().Foreground(lipgloss.Color("109")),
		value:   lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
		ok:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
		warn:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		bad:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")),
		accent:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		error:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")),
		help:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		mono:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		loading: lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchCmd(m.fetch, m.timeout), pollCmd(m.interval), clockCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
	case pollTickMsg:
		m.nextFetchAt = v.at.UTC().Add(m.interval)
		cmds := []tea.Cmd{pollCmd(m.interval)}
		if !m.fetching {
			m.fetching = true
			cmds = append(cmds, fetchCmd(m.fetch, m.timeout))
		}
		return m, tea.Batch(cmds...)
	case clockTickMsg:
		m.now = v.at.UTC()
		return m, clockCmd()
	case fetchResultMsg:
		m.fetching = false
		m.lastAttemptAt = v.at.UTC()
		m.lastFetchDuration = v.duration
		if v.err != nil {
			m.lastError = v.err.Error()
			return m, nil
		}
		m.lastError = ""
		m.lastSuccessAt = v.at.UTC()
		m.summary = v.summary
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "initializing..."
	}

	header := m.renderHeader()
	body := m.renderBody()
	exitHint := m.styles.dim.Render("Ctrl+C to exit")

	combined := lipgloss.JoinVertical(lipgloss.Left, header, body, "", exitHint)
	return clipToViewport(combined, m.width, m.height)
}

func (m Model) renderHeader() string {
	title := m.styles.title.Render(" codex usage monitor ")

	stateText := "idle"
	stateStyle := m.styles.dim
	if m.fetching {
		stateText = "refreshing"
		stateStyle = m.styles.loading
	} else if m.lastError != "" {
		stateText = "error"
		stateStyle = m.styles.bad
	} else if m.summary != nil {
		stateText = "healthy"
		stateStyle = m.styles.ok
	}

	left := title + "  " + m.styles.label.Render("state: ") + stateStyle.Render(stateText)
	if !m.nextFetchAt.IsZero() {
		refreshText := "[next refresh in " + humanDuration(m.nextFetchAt.Sub(m.now)) + "]"
		left += " " + m.styles.dim.Render(refreshText)
	}
	right := m.styles.dim.Render("utc " + m.now.Format("2006-01-02 15:04:05"))
	line1 := joinWithPaddingKeepRight(left, right, m.width)
	return line1
}

func (m Model) renderBody() string {
	if m.summary == nil {
		if m.lastError != "" {
			msg := m.styles.error.Render("last error: " + m.lastError)
			return m.styles.panel.Width(max(20, m.width-4)).Render(msg)
		}
		return m.styles.panel.Width(max(20, m.width-4)).Render(m.styles.loading.Render("loading usage data..."))
	}

	contentWidth := max(20, m.width-4)
	leftPanelWidth := contentWidth
	rightPanelWidth := contentWidth

	var windowsBlock string
	if contentWidth >= 94 {
		panelOverhead := horizontalOverhead(m.styles.panel)
		panelWidth, spacerWidth := splitEqualPanelContentWidths(contentWidth, panelOverhead)
		spacer := strings.Repeat(" ", spacerWidth)
		leftPanelWidth = panelWidth
		rightPanelWidth = panelWidth
		leftPanel := m.renderWindowPanel("five-hour window", m.summary.PrimaryWindow, leftPanelWidth)
		rightPanel := m.renderWindowPanel("weekly window", m.summary.SecondaryWindow, rightPanelWidth)
		windowsBlock = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, spacer, rightPanel)
	} else {
		leftPanel := m.renderWindowPanel("five-hour window", m.summary.PrimaryWindow, leftPanelWidth)
		rightPanel := m.renderWindowPanel("weekly window", m.summary.SecondaryWindow, rightPanelWidth)
		windowsBlock = lipgloss.JoinVertical(lipgloss.Left, leftPanel, "", rightPanel)
	}

	metaLines := []string{}
	if m.summary.TotalAccounts > 0 {
		metaLines = append(metaLines, m.styles.label.Render("accounts: ")+m.styles.value.Render(fmt.Sprintf("%d detected, %d reachable", m.summary.TotalAccounts, m.summary.SuccessfulAccounts)))
	}
	if m.summary.AccountEmail != "" {
		metaLines = append(metaLines, m.styles.label.Render("current account: ")+m.styles.value.Render(m.summary.AccountEmail))
	}
	if m.summary.ObservedTokensStatus != "" {
		metaLines = append(metaLines, m.styles.label.Render("five-hour tokens (sum across accounts): ")+m.styles.value.Render(formatObservedWindowCompact(m.summary.ObservedWindow5h, m.summary.ObservedTokens5h)))
		metaLines = append(metaLines, m.styles.label.Render("weekly tokens (sum across accounts): ")+m.styles.value.Render(formatObservedWindowCompact(m.summary.ObservedWindowWeekly, m.summary.ObservedTokensWeekly)))
	}
	if len(m.summary.Warnings) > 0 {
		for _, w := range m.summary.Warnings {
			metaLines = append(metaLines, m.styles.warn.Render("warning: "+w))
		}
	}
	if m.lastError != "" {
		metaLines = append(metaLines, m.styles.error.Render("last error: "+m.lastError))
	}
	for i := range metaLines {
		metaLines[i] = truncateRunes(metaLines[i], max(8, contentWidth-4))
	}

	metaPanel := m.styles.panel.Width(contentWidth).Render(strings.Join(metaLines, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, windowsBlock, metaPanel)
}

func (m Model) renderWindowPanel(title string, win usage.WindowSummary, maxWidth int) string {
	statusStyle := percentStyle(win.UsedPercent, m.styles)

	reset := "unknown"
	if win.ResetsAt != nil {
		reset = win.ResetsAt.Format("2006-01-02 15:04:05 UTC")
	}
	remaining := "unknown"
	if win.SecondsUntilReset != nil {
		if *win.SecondsUntilReset <= 0 {
			remaining = "resetting"
		} else {
			remaining = humanDuration(time.Duration(*win.SecondsUntilReset) * time.Second)
		}
	}

	lines := []string{
		m.styles.accent.Render(title),
		m.styles.label.Render("used: ") + statusStyle.Render(fmt.Sprintf("%d%%", win.UsedPercent)),
		m.styles.label.Render("resets at: ") + m.styles.value.Render(reset),
		m.styles.label.Render("resets in: ") + m.styles.value.Render(remaining),
	}
	return m.styles.panel.Width(max(20, maxWidth)).Render(strings.Join(lines, "\n"))
}

func percentStyle(percent int, styles styles) lipgloss.Style {
	switch {
	case percent >= 90:
		return styles.bad
	case percent >= 70:
		return styles.warn
	default:
		return styles.ok
	}
}

func percentBar(percent int, width int) string {
	if width < 4 {
		width = 4
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(math.Round(float64(percent) * float64(width) / 100.0))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat(".", width-filled) + "]"
}

func formatObservedWindowCompact(win *usage.ObservedTokenBreakdown, fallbackTotal *int64) string {
	if win == nil {
		if fallbackTotal == nil {
			return "n/a"
		}
		return compactCount(*fallbackTotal)
	}
	return compactCount(win.Total)
}

func compactCount(v int64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	if v < 1000 {
		return fmt.Sprintf("%s%d", sign, v)
	}
	units := []string{"", "k", "m", "b", "t"}
	value := float64(v)
	unitIndex := 0
	for value >= 1000 && unitIndex < len(units)-1 {
		value /= 1000
		unitIndex++
	}
	return fmt.Sprintf("%s%d%s", sign, int(math.Round(value)), units[unitIndex])
}

func pollCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return pollTickMsg{at: t}
	})
}

func clockCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return clockTickMsg{at: t}
	})
}

func fetchCmd(fetch FetchFunc, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		summary, err := fetch(ctx)
		return fetchResultMsg{
			at:       time.Now(),
			duration: time.Since(start),
			summary:  summary,
			err:      err,
		}
	}
}

func Run(opts Options) error {
	model := NewModel(opts)
	progOpts := []tea.ProgramOption{}
	if opts.AltScreen {
		progOpts = append(progOpts, tea.WithAltScreen())
	}
	prog := tea.NewProgram(model, progOpts...)
	_, err := prog.Run()
	return err
}

func joinWithPadding(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	padding := width - leftWidth - rightWidth
	if padding < 1 {
		return truncateRunes(left+" "+right, width)
	}
	return left + strings.Repeat(" ", padding) + right
}

func joinWithPaddingKeepRight(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	rightWidth := lipgloss.Width(right)
	if rightWidth >= width {
		return truncateRunes(right, width)
	}
	maxLeftWidth := width - rightWidth - 1
	if maxLeftWidth < 0 {
		maxLeftWidth = 0
	}
	left = truncateRunes(left, maxLeftWidth)
	leftWidth := lipgloss.Width(left)
	padding := width - leftWidth - rightWidth
	if padding < 1 {
		padding = 1
	}
	return left + strings.Repeat(" ", padding) + right
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	return ansi.Truncate(s, maxRunes, "")
}

func clipToViewport(s string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = truncateRunes(lines[i], width)
		pad := width - lipgloss.Width(lines[i])
		if pad > 0 {
			lines[i] += strings.Repeat(" ", pad)
		}
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return d.String()
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func splitEqualPanelContentWidths(contentWidth, panelOverhead int) (panelWidth int, spacerWidth int) {
	if contentWidth <= 0 {
		return 0, 0
	}
	// Keep panel content widths equal while ensuring:
	// 2*(panel content + panel overhead) + spacer == bottom panel outer width.
	usable := contentWidth - panelOverhead
	if usable < 3 {
		return 1, 1
	}
	if usable%2 == 0 {
		spacerWidth = 2
	} else {
		spacerWidth = 1
	}
	panelWidth = (usable - spacerWidth) / 2
	if panelWidth < 1 {
		panelWidth = 1
	}
	return panelWidth, spacerWidth
}

func horizontalOverhead(style lipgloss.Style) int {
	// Probe with a stable non-trivial width to avoid edge-case minimum sizing.
	const probeWidth = 40
	overhead := lipgloss.Width(style.Width(probeWidth).Render("")) - probeWidth
	if overhead < 0 {
		return 0
	}
	return overhead
}
