package usage

import (
	"context"
	"errors"
	"strings"
	"testing"
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
