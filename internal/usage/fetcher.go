package usage

import (
	"context"
	"fmt"
)

type Fetcher struct {
	primary  Source
	fallback Source
}

func NewDefaultFetcher() *Fetcher {
	return &Fetcher{
		primary:  NewAppServerSource(),
		fallback: NewOAuthSource(),
	}
}

func (f *Fetcher) Fetch(ctx context.Context) (*Summary, error) {
	if f.primary == nil {
		return nil, fmt.Errorf("missing primary source")
	}

	primarySummary, primaryErr := f.primary.Fetch(ctx)
	if primaryErr == nil {
		return primarySummary, nil
	}

	if f.fallback == nil {
		return nil, fmt.Errorf("primary source %q failed: %w", f.primary.Name(), primaryErr)
	}

	fallbackSummary, fallbackErr := f.fallback.Fetch(ctx)
	if fallbackErr == nil {
		fallbackSummary.Warnings = append(fallbackSummary.Warnings, fmt.Sprintf("primary source %q failed: %v", f.primary.Name(), primaryErr))
		return fallbackSummary, nil
	}

	return nil, fmt.Errorf(
		"primary source %q failed: %v; fallback source %q failed: %v",
		f.primary.Name(), primaryErr, f.fallback.Name(), fallbackErr,
	)
}

func (f *Fetcher) Close() error {
	var firstErr error
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
