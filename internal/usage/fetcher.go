package usage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Fetcher struct {
	primary  Source
	fallback Source

	accounts                []accountFetcher
	observed                tokenEstimator
	initializationNote      string
	accountLoader           func() ([]MonitorAccount, string, error)
	accountRefreshInterval  time.Duration
	accountsLastRefreshedAt time.Time
}

type accountFetcher struct {
	account  MonitorAccount
	primary  Source
	fallback Source
}

type accountFetchResult struct {
	index               int
	account             AccountSummary
	snapshot            *Summary
	fetchErr            error
	observedAvailable   bool
	observedUnavailable bool
	warnings            []string
}

type tokenEstimator interface {
	Estimate(codexHome string, now time.Time) (ObservedTokenEstimate, error)
}

func NewDefaultFetcher() *Fetcher {
	return newConfiguredFetcher(true)
}

func NewSnapshotFetcher() *Fetcher {
	return newConfiguredFetcher(false)
}

func newConfiguredFetcher(asyncObserved bool) *Fetcher {
	f := &Fetcher{
		observed:               newObservedTokenEstimator(60*time.Second, asyncObserved),
		accountLoader:          loadMonitorAccounts,
		accountRefreshInterval: 60 * time.Second,
	}
	f.refreshAccounts(time.Now().UTC(), true)
	return f
}

func (f *Fetcher) Fetch(ctx context.Context) (*Summary, error) {
	if len(f.accounts) > 0 {
		return f.fetchMultiAccount(ctx)
	}
	return f.fetchSingle(ctx)
}

func (f *Fetcher) fetchSingle(ctx context.Context) (*Summary, error) {
	if f.primary == nil {
		return nil, fmt.Errorf("missing primary source")
	}

	primarySummary, primaryErr := fetchWithFallback(ctx, f.primary, f.fallback)
	if primaryErr != nil {
		return nil, primaryErr
	}
	return primarySummary, nil
}

func (f *Fetcher) fetchMultiAccount(ctx context.Context) (*Summary, error) {
	now := time.Now().UTC()
	f.refreshAccounts(now, false)

	out := &Summary{
		TotalAccounts:        len(f.accounts),
		SuccessfulAccounts:   0,
		ObservedTokensStatus: observedTokensStatusUnavailable,
		FetchedAt:            now,
	}
	if f.initializationNote != "" {
		out.Warnings = append(out.Warnings, f.initializationNote)
	}

	var selectedSuccess *Summary
	selectedLabel := ""
	planTypes := map[string]struct{}{}

	anyAccountSuccess := false
	anyObservedAvailable := false
	unavailableObservedCount := 0
	var observedAnon observedWindowPair
	seenObservedByIdentity := map[string]observedWindowPair{}
	duplicateIdentityDetected := false

	results := f.fetchAccountsConcurrent(ctx, now)
	for _, result := range results {
		accountOut := result.account
		if result.fetchErr != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("account %q fetch failed: %v", accountOut.Label, result.fetchErr))
		} else if result.snapshot != nil {
			out.SuccessfulAccounts++
			anyAccountSuccess = true
			if accountOut.PlanType != "" {
				planTypes[accountOut.PlanType] = struct{}{}
			}
			if shouldSelectSummary(selectedSuccess, result.snapshot) {
				selectedSuccess = result.snapshot
				selectedLabel = accountOut.Label
			}
		}
		if result.observedAvailable {
			anyObservedAvailable = true
			pair := observedWindowPair{}
			if accountOut.ObservedWindow5h != nil {
				pair.Window5h = *accountOut.ObservedWindow5h
			}
			if accountOut.ObservedWindowWeekly != nil {
				pair.WindowWeekly = *accountOut.ObservedWindowWeekly
			}

			identity := identityKey(accountOut.AccountID, accountOut.UserID, accountOut.AccountEmail)
			if identity == "" {
				observedAnon = addObservedPairs(observedAnon, pair)
			} else {
				prev, seen := seenObservedByIdentity[identity]
				next := mergeObservedPairMax(prev, pair)
				if seen {
					duplicateIdentityDetected = true
				}
				seenObservedByIdentity[identity] = next
			}
		}
		if result.observedUnavailable {
			unavailableObservedCount++
		}
		out.Warnings = append(out.Warnings, result.warnings...)
		out.Accounts = append(out.Accounts, accountOut)
	}

	if selectedSuccess != nil {
		out.Source = selectedSuccess.Source
		out.PlanType = selectedSuccess.PlanType
		if len(planTypes) > 1 {
			out.PlanType = "mixed"
		}
		out.AccountEmail = selectedSuccess.AccountEmail
		out.AccountID = selectedSuccess.AccountID
		out.UserID = selectedSuccess.UserID
		out.PrimaryWindow = selectedSuccess.PrimaryWindow
		out.SecondaryWindow = selectedSuccess.SecondaryWindow
		out.WindowAccountLabel = selectedLabel
		out.AdditionalLimitCount = selectedSuccess.AdditionalLimitCount
		out.FetchedAt = selectedSuccess.FetchedAt
	}

	if anyObservedAvailable {
		observedTotal := observedAnon
		for _, pair := range seenObservedByIdentity {
			observedTotal = addObservedPairs(observedTotal, pair)
		}
		out.ObservedTokensStatus = observedTokensStatusEstimated
		out.ObservedWindow5h = &observedTotal.Window5h
		out.ObservedWindowWeekly = &observedTotal.WindowWeekly
		out.ObservedTokens5h = int64Ptr(observedTotal.Window5h.Total)
		out.ObservedTokensWeekly = int64Ptr(observedTotal.WindowWeekly.Total)
		out.ObservedTokensNote = "summed across all detected accounts"
		if unavailableObservedCount > 0 {
			out.ObservedTokensStatus = observedTokensStatusPartial
			out.ObservedTokensNote = "partial sum across detected accounts; some account homes unavailable"
		}
		if duplicateIdentityDetected {
			out.Warnings = append(out.Warnings, "duplicate account identity detected; token totals were deduplicated to avoid double counting")
		}
	} else if unavailableObservedCount > 0 {
		out.ObservedTokensStatus = observedTokensStatusUnavailable
		out.ObservedTokensNote = "token estimate warming or unavailable"
	}

	out.Warnings = dedupeStrings(out.Warnings)

	if !anyAccountSuccess && !anyObservedAvailable {
		return nil, fmt.Errorf("all account fetches failed and observed tokens are unavailable")
	}
	return out, nil
}

func fetchWithFallback(ctx context.Context, primary Source, fallback Source) (*Summary, error) {
	if primary == nil {
		return nil, fmt.Errorf("missing primary source")
	}

	primarySummary, primaryErr := primary.Fetch(ctx)
	if primaryErr == nil {
		return primarySummary, nil
	}

	if fallback == nil {
		return nil, fmt.Errorf("primary source %q failed: %w", primary.Name(), primaryErr)
	}

	fallbackSummary, fallbackErr := fallback.Fetch(ctx)
	if fallbackErr == nil {
		fallbackSummary.Warnings = append(fallbackSummary.Warnings, fmt.Sprintf("primary source %q failed: %v", primary.Name(), primaryErr))
		return fallbackSummary, nil
	}

	return nil, fmt.Errorf(
		"primary source %q failed: %v; fallback source %q failed: %v",
		primary.Name(), primaryErr, fallback.Name(), fallbackErr,
	)
}

func (f *Fetcher) Close() error {
	var firstErr error
	for _, account := range f.accounts {
		if account.primary != nil {
			if err := account.primary.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if account.fallback != nil {
			if err := account.fallback.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	if f.primary != nil {
		if err := f.primary.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if f.fallback != nil {
		if err := f.fallback.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (f *Fetcher) Primary() Source {
	return f.primary
}

func (f *Fetcher) Fallback() Source {
	return f.fallback
}

func int64Ptr(v int64) *int64 {
	out := v
	return &out
}

func shouldSelectSummary(current *Summary, candidate *Summary) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	if candidate.SecondaryWindow.UsedPercent != current.SecondaryWindow.UsedPercent {
		return candidate.SecondaryWindow.UsedPercent > current.SecondaryWindow.UsedPercent
	}
	if candidate.PrimaryWindow.UsedPercent != current.PrimaryWindow.UsedPercent {
		return candidate.PrimaryWindow.UsedPercent > current.PrimaryWindow.UsedPercent
	}
	return candidate.FetchedAt.After(current.FetchedAt)
}

func (f *Fetcher) refreshAccounts(now time.Time, force bool) {
	if f.accountLoader == nil {
		return
	}
	if !force && f.accountRefreshInterval > 0 && !f.accountsLastRefreshedAt.IsZero() {
		if now.Sub(f.accountsLastRefreshedAt) < f.accountRefreshInterval {
			return
		}
	}

	accounts, warning, err := f.accountLoader()
	f.accountsLastRefreshedAt = now
	if err != nil {
		f.initializationNote = err.Error()
		return
	}
	if len(accounts) == 0 {
		home, homeErr := defaultCodexHome()
		if homeErr == nil {
			accounts = []MonitorAccount{{Label: "default", CodexHome: home}}
		}
	}

	f.initializationNote = warning
	f.replaceAccountFetchers(accounts)
}

func (f *Fetcher) replaceAccountFetchers(accounts []MonitorAccount) {
	existingByHome := map[string]accountFetcher{}
	for _, account := range f.accounts {
		home := normalizeHome(account.account.CodexHome)
		if home == "" {
			continue
		}
		existingByHome[home] = account
	}

	usedHomes := map[string]struct{}{}
	next := make([]accountFetcher, 0, len(accounts))
	for _, account := range accounts {
		home := normalizeHome(account.CodexHome)
		if home == "" {
			continue
		}
		account.CodexHome = home
		if existing, ok := existingByHome[home]; ok {
			existing.account = account
			next = append(next, existing)
			usedHomes[home] = struct{}{}
			continue
		}

		next = append(next, accountFetcher{
			account:  account,
			primary:  NewAppServerSourceForHome(home),
			fallback: NewOAuthSourceForHome(home),
		})
		usedHomes[home] = struct{}{}
	}

	for home, existing := range existingByHome {
		if _, ok := usedHomes[home]; ok {
			continue
		}
		if existing.primary != nil {
			_ = existing.primary.Close()
		}
		if existing.fallback != nil {
			_ = existing.fallback.Close()
		}
	}
	f.accounts = next
}

func normalizeHome(home string) string {
	trimmed := strings.TrimSpace(home)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func identityKey(accountID, userID, email string) string {
	if v := strings.TrimSpace(accountID); v != "" {
		return "account:" + strings.ToLower(v)
	}
	if v := strings.TrimSpace(userID); v != "" {
		return "user:" + strings.ToLower(v)
	}
	if v := strings.TrimSpace(email); v != "" {
		return "email:" + strings.ToLower(v)
	}
	return ""
}

func addObservedPairs(a, b observedWindowPair) observedWindowPair {
	return observedWindowPair{
		Window5h:     addBreakdowns(a.Window5h, b.Window5h),
		WindowWeekly: addBreakdowns(a.WindowWeekly, b.WindowWeekly),
	}
}

func addBreakdowns(a, b ObservedTokenBreakdown) ObservedTokenBreakdown {
	return ObservedTokenBreakdown{
		Total:           a.Total + b.Total,
		Input:           a.Input + b.Input,
		CachedInput:     a.CachedInput + b.CachedInput,
		Output:          a.Output + b.Output,
		ReasoningOutput: a.ReasoningOutput + b.ReasoningOutput,
		CachedOutput:    a.CachedOutput + b.CachedOutput,
		HasSplit:        a.HasSplit || b.HasSplit,
		HasCachedOutput: a.HasCachedOutput || b.HasCachedOutput,
	}
}

func mergeObservedPairMax(prev, next observedWindowPair) observedWindowPair {
	return observedWindowPair{
		Window5h:     mergeBreakdownMax(prev.Window5h, next.Window5h),
		WindowWeekly: mergeBreakdownMax(prev.WindowWeekly, next.WindowWeekly),
	}
}

func mergeBreakdownMax(a, b ObservedTokenBreakdown) ObservedTokenBreakdown {
	if b.Total > a.Total {
		return b
	}
	return a
}

func (f *Fetcher) fetchAccountsConcurrent(ctx context.Context, now time.Time) []accountFetchResult {
	if len(f.accounts) == 0 {
		return nil
	}

	results := make([]accountFetchResult, len(f.accounts))
	parallelism := len(f.accounts)
	if parallelism > 4 {
		parallelism = 4
	}

	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup

	for i, account := range f.accounts {
		i := i
		account := account
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = f.fetchAccountResult(ctx, account, now, i)
		}()
	}
	wg.Wait()
	return results
}

func (f *Fetcher) fetchAccountResult(ctx context.Context, account accountFetcher, now time.Time, index int) accountFetchResult {
	result := accountFetchResult{
		index: index,
		account: AccountSummary{
			Label: account.account.Label,
		},
	}

	snapshot, fetchErr := fetchWithFallback(ctx, account.primary, account.fallback)
	if fetchErr != nil {
		result.fetchErr = fetchErr
		result.account.Error = fetchErr.Error()
	} else {
		result.snapshot = snapshot
		result.account.Source = snapshot.Source
		result.account.PlanType = snapshot.PlanType
		result.account.AccountEmail = snapshot.AccountEmail
		result.account.AccountID = snapshot.AccountID
		result.account.UserID = snapshot.UserID
		result.account.PrimaryWindow = snapshot.PrimaryWindow
		result.account.SecondaryWindow = snapshot.SecondaryWindow
		result.account.AdditionalLimitCount = snapshot.AdditionalLimitCount
		result.account.Warnings = append(result.account.Warnings, snapshot.Warnings...)
		ts := snapshot.FetchedAt
		result.account.FetchedAt = &ts
	}

	if f.observed != nil {
		estimate, estimateErr := f.observed.Estimate(account.account.CodexHome, now)
		if estimateErr != nil {
			result.account.ObservedTokensStatus = observedTokensStatusUnavailable
			result.account.ObservedTokensNote = estimate.Note
			result.observedUnavailable = true
			result.warnings = append(result.warnings, fmt.Sprintf("account %q observed tokens unavailable: %v", account.account.Label, estimateErr))
		} else {
			result.account.ObservedTokensStatus = estimate.Status
			result.account.ObservedTokensNote = estimate.Note
			result.account.Warnings = append(result.account.Warnings, estimate.Warnings...)
			result.account.ObservedWindow5h = &estimate.Window5h
			result.account.ObservedWindowWeekly = &estimate.WindowWeekly
			result.account.ObservedTokens5h = int64Ptr(estimate.Window5h.Total)
			result.account.ObservedTokensWeekly = int64Ptr(estimate.WindowWeekly.Total)

			if estimate.Status == observedTokensStatusUnavailable {
				result.observedUnavailable = true
			} else {
				result.observedAvailable = true
			}
		}
	}

	result.account.Warnings = dedupeStrings(result.account.Warnings)
	return result
}
