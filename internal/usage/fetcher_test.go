package usage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSource struct {
	name   string
	out    *Summary
	err    error
	closed bool
}

func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Fetch(context.Context) (*Summary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}
func (f *fakeSource) Close() error {
	f.closed = true
	return nil
}

func TestFetcherUsesPrimaryOnSuccess(t *testing.T) {
	primary := &fakeSource{name: "primary", out: &Summary{Source: "primary"}}
	fallback := &fakeSource{name: "fallback", out: &Summary{Source: "fallback"}}
	f := &Fetcher{primary: primary, fallback: fallback}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Source != "primary" {
		t.Fatalf("expected primary source, got %s", out.Source)
	}
}

func TestFetcherFallsBackWithWarning(t *testing.T) {
	primary := &fakeSource{name: "primary", err: errors.New("boom")}
	fallback := &fakeSource{name: "fallback", out: &Summary{Source: "fallback"}}
	f := &Fetcher{primary: primary, fallback: fallback}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Source != "fallback" {
		t.Fatalf("expected fallback source, got %s", out.Source)
	}
	if len(out.Warnings) == 0 {
		t.Fatalf("expected warning from primary failure")
	}
	if !strings.Contains(out.Warnings[0], "primary") {
		t.Fatalf("warning should mention primary failure")
	}
}

func TestFetcherFailsWhenBothSourcesFail(t *testing.T) {
	primary := &fakeSource{name: "primary", err: errors.New("p")}
	fallback := &fakeSource{name: "fallback", err: errors.New("f")}
	f := &Fetcher{primary: primary, fallback: fallback}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "primary") || !strings.Contains(err.Error(), "fallback") {
		t.Fatalf("expected error to include both sources: %v", err)
	}
}

func TestFetcherCloseClosesAllSources(t *testing.T) {
	primary := &fakeSource{name: "primary"}
	fallback := &fakeSource{name: "fallback"}
	f := &Fetcher{primary: primary, fallback: fallback}

	if err := f.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !primary.closed || !fallback.closed {
		t.Fatalf("expected both sources to close")
	}
}

type fakeEstimator struct {
	values map[string]ObservedTokenEstimate
	errs   map[string]error
}

func (f fakeEstimator) Estimate(codexHome string, _ time.Time) (ObservedTokenEstimate, error) {
	if err, ok := f.errs[codexHome]; ok {
		return ObservedTokenEstimate{
			Status: observedTokensStatusUnavailable,
			Note:   err.Error(),
		}, err
	}
	v, ok := f.values[codexHome]
	if !ok {
		return ObservedTokenEstimate{
			Status: observedTokensStatusUnavailable,
			Note:   "missing estimate",
		}, errors.New("missing estimate")
	}
	return v, nil
}

func TestFetcherAggregatesMultiAccountObservedTokens(t *testing.T) {
	primaryA := &fakeSource{name: "primary-a", out: &Summary{
		Source:          "app-server",
		PlanType:        "pro",
		AccountEmail:    "a@example.com",
		PrimaryWindow:   WindowSummary{UsedPercent: 20},
		SecondaryWindow: WindowSummary{UsedPercent: 50},
	}}
	primaryB := &fakeSource{name: "primary-b", err: errors.New("boom")}
	fallbackB := &fakeSource{name: "fallback-b", out: &Summary{
		Source:          "oauth",
		PlanType:        "pro",
		AccountEmail:    "b@example.com",
		PrimaryWindow:   WindowSummary{UsedPercent: 60},
		SecondaryWindow: WindowSummary{UsedPercent: 70},
	}}

	f := &Fetcher{
		accounts: []accountFetcher{
			{
				account:  MonitorAccount{Label: "a", CodexHome: "/a"},
				primary:  primaryA,
				fallback: &fakeSource{name: "fallback-a"},
			},
			{
				account:  MonitorAccount{Label: "b", CodexHome: "/b"},
				primary:  primaryB,
				fallback: fallbackB,
			},
		},
		observed: fakeEstimator{
			values: map[string]ObservedTokenEstimate{
				"/a": {
					Window5h:     ObservedTokenBreakdown{Total: 100},
					WindowWeekly: ObservedTokenBreakdown{Total: 200},
					Status:       observedTokensStatusEstimated,
				},
				"/b": {
					Window5h:     ObservedTokenBreakdown{Total: 30},
					WindowWeekly: ObservedTokenBreakdown{Total: 80},
					Status:       observedTokensStatusEstimated,
				},
			},
		},
	}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalAccounts != 2 || out.SuccessfulAccounts != 2 {
		t.Fatalf("expected 2/2 account success, got %d/%d", out.SuccessfulAccounts, out.TotalAccounts)
	}
	if out.ObservedTokens5h == nil || *out.ObservedTokens5h != 130 {
		t.Fatalf("expected aggregated 5h observed total, got %+v", out.ObservedTokens5h)
	}
	if out.ObservedTokensWeekly == nil || *out.ObservedTokensWeekly != 280 {
		t.Fatalf("expected aggregated weekly observed total, got %+v", out.ObservedTokensWeekly)
	}
	if out.ObservedTokensStatus != observedTokensStatusEstimated {
		t.Fatalf("expected estimated observed status, got %q", out.ObservedTokensStatus)
	}
	if len(out.Accounts) != 2 {
		t.Fatalf("expected 2 account rows, got %d", len(out.Accounts))
	}
	if out.Accounts[1].Source != "oauth" {
		t.Fatalf("expected fallback source for account b, got %q", out.Accounts[1].Source)
	}
	if out.Accounts[0].ObservedTokens5h == nil || *out.Accounts[0].ObservedTokens5h != 100 {
		t.Fatalf("expected account a observed 5h total")
	}
	if out.Accounts[1].ObservedTokens5h == nil || *out.Accounts[1].ObservedTokens5h != 30 {
		t.Fatalf("expected account b observed 5h total")
	}
	if out.SecondaryWindow.UsedPercent != 70 {
		t.Fatalf("expected top window summary from highest-pressure account")
	}
	if out.WindowAccountLabel != "b" {
		t.Fatalf("expected window account label b, got %q", out.WindowAccountLabel)
	}
}

func TestFetcherAllowsObservedOnlyWhenAllSourcesFail(t *testing.T) {
	f := &Fetcher{
		accounts: []accountFetcher{
			{
				account:  MonitorAccount{Label: "a", CodexHome: "/a"},
				primary:  &fakeSource{name: "primary-a", err: errors.New("p")},
				fallback: &fakeSource{name: "fallback-a", err: errors.New("f")},
			},
		},
		observed: fakeEstimator{
			values: map[string]ObservedTokenEstimate{
				"/a": {
					Window5h:     ObservedTokenBreakdown{Total: 12},
					WindowWeekly: ObservedTokenBreakdown{Total: 99},
					Status:       observedTokensStatusEstimated,
				},
			},
		},
	}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.SuccessfulAccounts != 0 {
		t.Fatalf("expected zero successful accounts")
	}
	if out.ObservedTokensStatus != observedTokensStatusEstimated {
		t.Fatalf("expected observed estimate status")
	}
	if out.ObservedTokens5h == nil || *out.ObservedTokens5h != 12 {
		t.Fatalf("expected observed-only totals at summary level")
	}
}

func TestFetcherMarksObservedPartialWhenSomeAccountsUnavailable(t *testing.T) {
	f := &Fetcher{
		accounts: []accountFetcher{
			{
				account:  MonitorAccount{Label: "a", CodexHome: "/a"},
				primary:  &fakeSource{name: "primary-a", out: &Summary{PrimaryWindow: WindowSummary{}, SecondaryWindow: WindowSummary{}}},
				fallback: &fakeSource{name: "fallback-a"},
			},
			{
				account:  MonitorAccount{Label: "b", CodexHome: "/b"},
				primary:  &fakeSource{name: "primary-b", out: &Summary{PrimaryWindow: WindowSummary{}, SecondaryWindow: WindowSummary{}}},
				fallback: &fakeSource{name: "fallback-b"},
			},
		},
		observed: fakeEstimator{
			values: map[string]ObservedTokenEstimate{
				"/a": {
					Window5h:     ObservedTokenBreakdown{Total: 10},
					WindowWeekly: ObservedTokenBreakdown{Total: 20},
					Status:       observedTokensStatusEstimated,
				},
			},
			errs: map[string]error{
				"/b": errors.New("missing logs"),
			},
		},
	}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ObservedTokensStatus != observedTokensStatusPartial {
		t.Fatalf("expected partial observed status, got %q", out.ObservedTokensStatus)
	}
	if out.ObservedTokens5h == nil || *out.ObservedTokens5h != 10 {
		t.Fatalf("expected partial observed 5h total from available accounts")
	}
}

func TestFetcherDeduplicatesObservedTotalsByIdentity(t *testing.T) {
	f := &Fetcher{
		accounts: []accountFetcher{
			{
				account: MonitorAccount{Label: "a", CodexHome: "/a"},
				primary: &fakeSource{name: "primary-a", out: &Summary{
					AccountEmail:    "same@example.com",
					PrimaryWindow:   WindowSummary{UsedPercent: 10},
					SecondaryWindow: WindowSummary{UsedPercent: 20},
				}},
				fallback: &fakeSource{name: "fallback-a"},
			},
			{
				account: MonitorAccount{Label: "b", CodexHome: "/b"},
				primary: &fakeSource{name: "primary-b", out: &Summary{
					AccountEmail:    "same@example.com",
					PrimaryWindow:   WindowSummary{UsedPercent: 30},
					SecondaryWindow: WindowSummary{UsedPercent: 40},
				}},
				fallback: &fakeSource{name: "fallback-b"},
			},
		},
		observed: fakeEstimator{
			values: map[string]ObservedTokenEstimate{
				"/a": {
					Window5h:     ObservedTokenBreakdown{Total: 100},
					WindowWeekly: ObservedTokenBreakdown{Total: 200},
					Status:       observedTokensStatusEstimated,
				},
				"/b": {
					Window5h:     ObservedTokenBreakdown{Total: 150},
					WindowWeekly: ObservedTokenBreakdown{Total: 180},
					Status:       observedTokensStatusEstimated,
				},
			},
		},
	}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ObservedTokens5h == nil || *out.ObservedTokens5h != 150 {
		t.Fatalf("expected deduped 5h total by identity, got %+v", out.ObservedTokens5h)
	}
	if out.ObservedTokensWeekly == nil || *out.ObservedTokensWeekly != 200 {
		t.Fatalf("expected deduped weekly total by identity, got %+v", out.ObservedTokensWeekly)
	}
}

func TestReplaceAccountFetchersClosesRemovedHomes(t *testing.T) {
	oldPrimary := &fakeSource{name: "old-primary"}
	oldFallback := &fakeSource{name: "old-fallback"}
	f := &Fetcher{
		accounts: []accountFetcher{
			{
				account:  MonitorAccount{Label: "old", CodexHome: "/old"},
				primary:  oldPrimary,
				fallback: oldFallback,
			},
		},
	}

	f.replaceAccountFetchers([]MonitorAccount{
		{Label: "new", CodexHome: "/new"},
	})

	if !oldPrimary.closed || !oldFallback.closed {
		t.Fatalf("expected removed account sources to be closed")
	}
	if len(f.accounts) != 1 {
		t.Fatalf("expected one replacement account fetcher")
	}
	if f.accounts[0].account.Label != "new" {
		t.Fatalf("expected replacement account label")
	}
}

func TestRefreshAccountsReloadsAndReusesExistingHomes(t *testing.T) {
	callCount := 0
	f := &Fetcher{
		accountLoader: func() ([]MonitorAccount, string, error) {
			callCount++
			switch callCount {
			case 1:
				return []MonitorAccount{
					{Label: "alpha", CodexHome: "/alpha"},
				}, "", nil
			default:
				return []MonitorAccount{
					{Label: "alpha-renamed", CodexHome: "/alpha"},
					{Label: "beta", CodexHome: "/beta"},
				}, "", nil
			}
		},
		accountRefreshInterval: time.Minute,
	}

	start := time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC)
	f.refreshAccounts(start, true)
	if len(f.accounts) != 1 {
		t.Fatalf("expected one initial account")
	}
	reusedPrimary := f.accounts[0].primary

	f.refreshAccounts(start.Add(2*time.Minute), false)
	if len(f.accounts) != 2 {
		t.Fatalf("expected second refresh to load two accounts")
	}

	var alpha accountFetcher
	for _, account := range f.accounts {
		if account.account.CodexHome == "/alpha" {
			alpha = account
			break
		}
	}
	if alpha.account.Label != "alpha-renamed" {
		t.Fatalf("expected refreshed label for reused home")
	}
	if alpha.primary != reusedPrimary {
		t.Fatalf("expected existing source to be reused for unchanged home")
	}
}
