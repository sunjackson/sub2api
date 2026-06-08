package admin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	oauthpkg "github.com/Wei-Shaw/sub2api/internal/pkg/oauth"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type statusCheckClaudeOAuthClient struct {
	refreshErr error
}

func (c *statusCheckClaudeOAuthClient) GetOrganizationUUID(ctx context.Context, sessionKey, proxyURL string) (string, error) {
	return "", nil
}

func (c *statusCheckClaudeOAuthClient) GetAuthorizationCode(ctx context.Context, sessionKey, orgUUID, scope, codeChallenge, state, proxyURL string) (string, error) {
	return "", nil
}

func (c *statusCheckClaudeOAuthClient) ExchangeCodeForToken(ctx context.Context, code, codeVerifier, state, proxyURL string, isSetupToken bool) (*oauthpkg.TokenResponse, error) {
	return nil, nil
}

func (c *statusCheckClaudeOAuthClient) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*oauthpkg.TokenResponse, error) {
	if c.refreshErr != nil {
		return nil, c.refreshErr
	}
	return &oauthpkg.TokenResponse{
		AccessToken:  "new-access-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "new-refresh-token",
		Scope:        "profile",
	}, nil
}

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

func TestStatusCheckTokenRefreshFailureMarksAccountError(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{
		adminService: adminSvc,
		oauthService: service.NewOAuthService(nil, &statusCheckClaudeOAuthClient{
			refreshErr: errors.New("invalid_grant"),
		}),
	}
	account := &service.Account{
		ID:       42,
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "old-access-token",
			"refresh_token": "old-refresh-token",
		},
	}

	_, attempted, _, err := handler.refreshAccountTokenForStatusCheck(context.Background(), account)
	if err == nil {
		t.Fatal("expected token refresh error")
	}
	if !attempted {
		t.Fatal("expected token refresh to be attempted")
	}

	adminSvc.mu.Lock()
	errorMessage := adminSvc.accountErrors[account.ID]
	adminSvc.mu.Unlock()
	if !strings.Contains(errorMessage, "token refresh failed: invalid_grant") {
		t.Fatalf("account error = %q, want token refresh failure", errorMessage)
	}
}

func TestStatusCheckTokenRefreshSkipsNonOAuthTokenAccounts(t *testing.T) {
	handler := &AccountHandler{}
	account := &service.Account{
		ID:       43,
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeSetupToken,
	}

	updated, attempted, _, err := handler.refreshAccountTokenForStatusCheck(context.Background(), account)
	if err != nil {
		t.Fatalf("refreshAccountTokenForStatusCheck returned error: %v", err)
	}
	if attempted {
		t.Fatal("setup token account should not attempt OAuth token refresh during status check")
	}
	if updated != account {
		t.Fatal("expected skipped account to be returned unchanged")
	}
}

func TestPersistBatchStatusCheckAccountStateClearsRecoveredErrorBeforeRateLimit(t *testing.T) {
	adminSvc := newStubAdminService()
	resetAt := time.Now().UTC().Add(5 * time.Hour)
	account := &service.Account{
		ID:           101,
		Status:       service.StatusError,
		ErrorMessage: "token refresh failed: invalid_grant",
	}

	updated, err := persistBatchStatusCheckAccountState(context.Background(), adminSvc, account, &resetAt)
	if err != nil {
		t.Fatalf("persistBatchStatusCheckAccountState returned error: %v", err)
	}
	if !updated {
		t.Fatal("expected account state to be updated")
	}

	adminSvc.mu.Lock()
	defer adminSvc.mu.Unlock()
	if len(adminSvc.clearedAccountIDs) != 1 || adminSvc.clearedAccountIDs[0] != account.ID {
		t.Fatalf("clearedAccountIDs = %#v, want [%d]", adminSvc.clearedAccountIDs, account.ID)
	}
	if len(adminSvc.rateLimitedAccountIDs) != 1 || adminSvc.rateLimitedAccountIDs[0] != account.ID {
		t.Fatalf("rateLimitedAccountIDs = %#v, want [%d]", adminSvc.rateLimitedAccountIDs, account.ID)
	}
	if got := adminSvc.rateLimitResetAtByID[account.ID]; !got.Equal(resetAt) {
		t.Fatalf("resetAt = %s, want %s", got.Format(time.RFC3339), resetAt.Format(time.RFC3339))
	}
}

func TestPersistBatchStatusCheckAccountStateClearsStaleRuntimeStateWhenHealthy(t *testing.T) {
	adminSvc := newStubAdminService()
	limitedAt := time.Now().UTC().Add(-time.Hour)
	resetAt := time.Now().UTC().Add(time.Hour)
	account := &service.Account{
		ID:                     102,
		Status:                 service.StatusActive,
		RateLimitedAt:          &limitedAt,
		RateLimitResetAt:       &resetAt,
		TempUnschedulableUntil: &resetAt,
		Extra: map[string]any{
			"model_rate_limits": map[string]any{"gpt-5": "blocked"},
		},
	}

	updated, err := persistBatchStatusCheckAccountState(context.Background(), adminSvc, account, nil)
	if err != nil {
		t.Fatalf("persistBatchStatusCheckAccountState returned error: %v", err)
	}
	if !updated {
		t.Fatal("expected stale runtime state to be cleared")
	}

	adminSvc.mu.Lock()
	defer adminSvc.mu.Unlock()
	if len(adminSvc.clearedAccountIDs) != 1 || adminSvc.clearedAccountIDs[0] != account.ID {
		t.Fatalf("clearedAccountIDs = %#v, want [%d]", adminSvc.clearedAccountIDs, account.ID)
	}
	if len(adminSvc.rateLimitedAccountIDs) != 0 {
		t.Fatalf("rateLimitedAccountIDs = %#v, want empty", adminSvc.rateLimitedAccountIDs)
	}
}

func TestPersistBatchStatusCheckAccountStateLeavesHealthyAndDisabledAccountsAlone(t *testing.T) {
	adminSvc := newStubAdminService()
	resetAt := time.Now().UTC().Add(time.Hour)

	for _, account := range []*service.Account{
		{ID: 103, Status: service.StatusActive},
		{ID: 104, Status: service.StatusDisabled, RateLimitResetAt: &resetAt},
	} {
		updated, err := persistBatchStatusCheckAccountState(context.Background(), adminSvc, account, nil)
		if err != nil {
			t.Fatalf("persistBatchStatusCheckAccountState(%d) returned error: %v", account.ID, err)
		}
		if updated {
			t.Fatalf("account %d updated unexpectedly", account.ID)
		}
	}

	adminSvc.mu.Lock()
	defer adminSvc.mu.Unlock()
	if len(adminSvc.clearedAccountIDs) != 0 {
		t.Fatalf("clearedAccountIDs = %#v, want empty", adminSvc.clearedAccountIDs)
	}
	if len(adminSvc.rateLimitedAccountIDs) != 0 {
		t.Fatalf("rateLimitedAccountIDs = %#v, want empty", adminSvc.rateLimitedAccountIDs)
	}
}
