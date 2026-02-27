package tui

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/olliecrow/codex_usage_monitor/internal/usage"
)

func TestViewFitsViewportAcrossSizes(t *testing.T) {
	sizes := []struct {
		width  int
		height int
	}{
		{60, 18},
		{80, 22},
		{100, 26},
		{140, 34},
	}

	for _, s := range sizes {
		t.Run(strconv.Itoa(s.width)+"x"+strconv.Itoa(s.height), func(t *testing.T) {
			m := seededModel()
			m.width = s.width
			m.height = s.height
			out := m.View()
			lines := strings.Split(out, "\n")
			if len(lines) != s.height {
				t.Fatalf("expected %d lines, got %d", s.height, len(lines))
			}
			for i, line := range lines {
				if lipgloss.Width(line) > s.width {
					t.Fatalf("line %d exceeded width: got %d max %d", i+1, lipgloss.Width(line), s.width)
				}
			}
		})
	}
}

func TestPercentBarBounds(t *testing.T) {
	cases := []int{-20, 0, 17, 50, 99, 100, 120}
	for _, percent := range cases {
		bar := percentBar(percent, 20)
		if !strings.HasPrefix(bar, "[") || !strings.HasSuffix(bar, "]") {
			t.Fatalf("unexpected bar format: %q", bar)
		}
		if lipgloss.Width(bar) != 22 {
			t.Fatalf("unexpected bar width: got %d", lipgloss.Width(bar))
		}
	}
}

func TestNarrowViewStillRendersCoreFields(t *testing.T) {
	m := seededModel()
	m.width = 42
	m.height = 14
	out := m.View()
	if !strings.Contains(out, "five-hour window") {
		t.Fatalf("expected five-hour section in output")
	}
	if !strings.Contains(out, "weekly window") {
		t.Fatalf("expected weekly section in output")
	}
}

func TestViewRendersAggregatedTokenSection(t *testing.T) {
	m := seededModel()
	m.width = 100
	m.height = 26
	total5h := int64(120000)
	total1w := int64(450000)
	m.summary.ObservedTokensStatus = "estimated"
	m.summary.ObservedTokens5h = &total5h
	m.summary.ObservedTokensWeekly = &total1w
	m.summary.ObservedWindow5h = &usage.ObservedTokenBreakdown{
		Total:       total5h,
		Input:       100000,
		CachedInput: 90000,
		Output:      20000,
		HasSplit:    true,
	}
	m.summary.ObservedWindowWeekly = &usage.ObservedTokenBreakdown{
		Total:       total1w,
		Input:       300000,
		CachedInput: 200000,
		Output:      150000,
		HasSplit:    true,
	}
	out := m.View()
	if !strings.Contains(out, "five-hour tokens (sum across accounts):") {
		t.Fatalf("expected aggregated five-hour token line in output")
	}
	if !strings.Contains(out, "weekly tokens (sum across accounts):") {
		t.Fatalf("expected aggregated weekly token line in output")
	}
	if strings.Contains(out, "estimated: ") {
		t.Fatalf("expected totals-only token display without split lines")
	}
}

func TestHeaderShowsCtrlCOnly(t *testing.T) {
	m := seededModel()
	m.width = 100
	m.height = 20
	header := m.renderHeader()
	if strings.Contains(header, "ctrl+c") || strings.Contains(header, "q quit") || strings.Contains(header, "r refresh") {
		t.Fatalf("expected header without interactive key hints, got: %q", header)
	}
}

func TestWindowPanelUsesResetsInLabel(t *testing.T) {
	m := seededModel()
	m.width = 100
	m.height = 20
	out := m.renderBody()
	if !strings.Contains(out, "resets in:") {
		t.Fatalf("expected resets in label in window panels")
	}
}

func TestWideLayoutPanelsAlignWidths(t *testing.T) {
	widths := []int{98, 99, 100, 101, 120, 121, 140}
	heights := []int{18, 24, 32}

	for _, w := range widths {
		for _, h := range heights {
			m := seededModel()
			m.width = w
			m.height = h
			body := m.renderBody()

			lines := strings.Split(body, "\n")
			topLine := ""
			metaTop := ""
			for _, line := range lines {
				if strings.Count(line, "╭") >= 2 && topLine == "" {
					topLine = line
					continue
				}
				if strings.Count(line, "╭") == 1 {
					metaTop = line
				}
			}
			if topLine == "" || metaTop == "" {
				t.Fatalf("expected top and metadata panel border lines for %dx%d", w, h)
			}
			topWidth := lipgloss.Width(topLine)
			metaWidth := lipgloss.Width(metaTop)
			if topWidth != metaWidth {
				t.Fatalf("expected aligned widths for %dx%d, got top=%d meta=%d", w, h, topWidth, metaWidth)
			}

			topRunes := []rune(topLine)
			firstRight := nthRuneIndex(topRunes, '╮', 1)
			secondLeft := nthRuneIndex(topRunes, '╭', 2)
			if firstRight < 0 || secondLeft < 0 || secondLeft <= firstRight {
				t.Fatalf("expected two top panels for %dx%d", w, h)
			}
			gapStart := firstRight + 1
			gapEnd := secondLeft - 1
			dividerCenter := (float64(gapStart) + float64(gapEnd)) / 2.0
			fullCenter := float64(topWidth-1) / 2.0
			if math.Abs(dividerCenter-fullCenter) > 0.5 {
				t.Fatalf("expected centered divider for %dx%d, divider=%.1f full=%.1f", w, h, dividerCenter, fullCenter)
			}
		}
	}
}

func TestHeaderIncludesRefreshBracketOnTopLine(t *testing.T) {
	m := seededModel()
	m.width = 100
	header := m.renderHeader()
	lines := strings.Split(header, "\n")
	if len(lines) != 1 {
		t.Fatalf("expected single-line header")
	}
	if !strings.Contains(lines[0], "[next refresh in ") {
		t.Fatalf("expected bracketed refresh countdown on header line")
	}
	if strings.Contains(lines[0], "interval") {
		t.Fatalf("did not expect interval label in header")
	}
	if lipgloss.Width(lines[0]) > m.width {
		t.Fatalf("header line exceeded width")
	}
}

func TestHeaderRetainsUTCTimestampAtNarrowWidth(t *testing.T) {
	m := seededModel()
	m.width = 58
	header := m.renderHeader()
	lines := strings.Split(header, "\n")
	if len(lines) != 1 {
		t.Fatalf("expected single-line header")
	}
	if !strings.Contains(lines[0], "utc 2026-02-26 15:00:00") {
		t.Fatalf("expected narrow header to retain utc timestamp, got: %q", lines[0])
	}
	if lipgloss.Width(lines[0]) > m.width {
		t.Fatalf("header line exceeded width")
	}
}

func TestViewportClippingHasNoEllipsisArtifacts(t *testing.T) {
	m := seededModel()
	m.width = 95
	m.height = 22
	out := m.View()
	if strings.Contains(out, "…") {
		t.Fatalf("expected no ellipsis clipping artifacts in viewport output")
	}
}

func TestViewShowsExitHintAtBottom(t *testing.T) {
	m := seededModel()
	m.width = 120
	m.height = 30
	out := m.View()
	if !strings.Contains(out, "Ctrl+C to exit") {
		t.Fatalf("expected bottom exit hint in view")
	}
	if strings.Contains(out, "last successful snapshot") {
		t.Fatalf("did not expect last successful snapshot footer line")
	}
}

func nthRuneIndex(runes []rune, target rune, n int) int {
	if n <= 0 {
		return -1
	}
	count := 0
	for i, r := range runes {
		if r == target {
			count++
			if count == n {
				return i
			}
		}
	}
	return -1
}

func seededModel() Model {
	now := time.Date(2026, 2, 26, 15, 0, 0, 0, time.UTC)
	reset1 := now.Add(90 * time.Minute)
	reset2 := now.Add(7 * 24 * time.Hour)
	sec1 := int64(90 * 60)
	sec2 := int64(7 * 24 * 60 * 60)
	m := NewModel(Options{
		Interval: 15 * time.Second,
		Timeout:  8 * time.Second,
		NoColor:  true,
		Fetch: func(_ context.Context) (*usage.Summary, error) {
			return nil, nil
		},
	})
	m.now = now
	m.fetching = false
	m.lastAttemptAt = now.Add(-2 * time.Second)
	m.lastSuccessAt = now.Add(-2 * time.Second)
	m.lastFetchDuration = 420 * time.Millisecond
	m.nextFetchAt = now.Add(13 * time.Second)
	m.summary = &usage.Summary{
		Source:       "app-server",
		PlanType:     "pro",
		AccountEmail: "me@example.com",
		PrimaryWindow: usage.WindowSummary{
			UsedPercent:       41,
			ResetsAt:          &reset1,
			SecondsUntilReset: &sec1,
		},
		SecondaryWindow: usage.WindowSummary{
			UsedPercent:       69,
			ResetsAt:          &reset2,
			SecondsUntilReset: &sec2,
		},
		FetchedAt: now.Add(-2 * time.Second),
	}
	return m
}
