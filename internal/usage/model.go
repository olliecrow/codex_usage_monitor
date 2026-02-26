package usage

import "time"

// Summary is the normalized subscription usage snapshot used by CLI and TUI.
type Summary struct {
	Source               string        `json:"source"`
	PlanType             string        `json:"plan_type"`
	AccountEmail         string        `json:"account_email,omitempty"`
	AccountID            string        `json:"account_id,omitempty"`
	UserID               string        `json:"user_id,omitempty"`
	PrimaryWindow        WindowSummary `json:"primary_window"`
	SecondaryWindow      WindowSummary `json:"secondary_window"`
	AdditionalLimitCount int           `json:"additional_limit_count,omitempty"`
	Warnings             []string      `json:"warnings,omitempty"`
	FetchedAt            time.Time     `json:"fetched_at"`
}

type WindowSummary struct {
	UsedPercent        int        `json:"used_percent"`
	WindowDurationMins *int       `json:"window_duration_mins,omitempty"`
	ResetsAt           *time.Time `json:"resets_at,omitempty"`
	SecondsUntilReset  *int64     `json:"seconds_until_reset,omitempty"`
}

type DoctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Details string `json:"details"`
}
