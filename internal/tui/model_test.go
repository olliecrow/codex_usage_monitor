package tui

import (
	"context"
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
	out := m.View()
	if !strings.Contains(out, "five-hour tokens (sum):") {
		t.Fatalf("expected aggregated five-hour token line in output")
	}
	if !strings.Contains(out, "weekly tokens (sum):") {
		t.Fatalf("expected aggregated weekly token line in output")
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
