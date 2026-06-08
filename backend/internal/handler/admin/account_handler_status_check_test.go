package admin

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestExhaustedUsageResetAtPicksLatestActiveExhaustedWindow(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	fiveHourReset := now.Add(2 * time.Hour)
	sevenDayReset := now.Add(6 * 24 * time.Hour)

	resetAt, windows := exhaustedUsageResetAt(&service.UsageInfo{
		FiveHour: &service.UsageProgress{
			Utilization: 100,
			ResetsAt:    &fiveHourReset,
		},
		SevenDay: &service.UsageProgress{
			Utilization: 100,
			ResetsAt:    &sevenDayReset,
		},
	}, now)

	if resetAt == nil {
		t.Fatal("expected exhausted usage to return a reset time")
	}
	if !resetAt.Equal(sevenDayReset) {
		t.Fatalf("resetAt = %s, want %s", resetAt.Format(time.RFC3339), sevenDayReset.Format(time.RFC3339))
	}
	if len(windows) != 2 || windows[0] != "5h" || windows[1] != "7d" {
		t.Fatalf("windows = %#v, want [5h 7d]", windows)
	}
}

func TestExhaustedUsageResetAtIgnoresUnderThresholdAndExpiredWindows(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	resetAt, windows := exhaustedUsageResetAt(&service.UsageInfo{
		FiveHour: &service.UsageProgress{
			Utilization: 99.9,
			ResetsAt:    &future,
		},
		SevenDay: &service.UsageProgress{
			Utilization: 100,
			ResetsAt:    &past,
		},
	}, now)

	if resetAt != nil {
		t.Fatalf("resetAt = %s, want nil", resetAt.Format(time.RFC3339))
	}
	if len(windows) != 0 {
		t.Fatalf("windows = %#v, want empty", windows)
	}
}
