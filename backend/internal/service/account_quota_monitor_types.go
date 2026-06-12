package service

import (
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	QuotaMonitorProviderSub2API = "sub2api"
	QuotaMonitorProviderNewAPI  = "newapi"
	QuotaMonitorProviderCustom  = "custom"

	QuotaMonitorStatusUnknown    = "unknown"
	QuotaMonitorStatusOK         = "ok"
	QuotaMonitorStatusLowBalance = "low_balance"
	QuotaMonitorStatusError      = "error"

	quotaMonitorMinIntervalSeconds      = 60
	quotaMonitorMaxIntervalSeconds      = 86400
	quotaMonitorDefaultCurrency         = "USD"
	quotaMonitorMessageMaxBytes         = 500
	quotaMonitorHTTPTimeout             = 20 * time.Second
	quotaMonitorResponseMaxBytes        = 256 * 1024
	quotaMonitorWorkerConcurrency       = 4
	quotaMonitorStartupLoadTimeout      = 10 * time.Second
	quotaMonitorRunOneTimeout           = 35 * time.Second
	quotaMonitorBatchDefaultMaxAccounts = 1000
	quotaMonitorBatchFetchPageSize      = 1000

	QuotaMonitorHistoryDefaultLimit = 100
	QuotaMonitorHistoryMaxLimit     = 1000
)

var (
	ErrAccountQuotaMonitorNotFound = infraerrors.NotFound(
		"ACCOUNT_QUOTA_MONITOR_NOT_FOUND", "account quota monitor not found",
	)
	ErrAccountQuotaMonitorInvalidProvider = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_INVALID_PROVIDER", "provider must be one of sub2api/newapi/custom",
	)
	ErrAccountQuotaMonitorInvalidInterval = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_INVALID_INTERVAL", "interval_seconds must be in [60, 86400]",
	)
	ErrAccountQuotaMonitorMissingName = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_MISSING_NAME", "name is required",
	)
	ErrAccountQuotaMonitorMissingAccount = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_MISSING_ACCOUNT", "account_id is required",
	)
	ErrAccountQuotaMonitorMissingEndpoint = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_MISSING_ENDPOINT", "endpoint is required or account credentials must include base_url",
	)
	ErrAccountQuotaMonitorMissingAPIKey = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_MISSING_API_KEY", "api key is required or account credentials must include api_key/access_token",
	)
	ErrAccountQuotaMonitorInvalidEndpoint = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_INVALID_ENDPOINT", "endpoint must be a valid https URL",
	)
	ErrAccountQuotaMonitorEndpointScheme = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_ENDPOINT_SCHEME", "endpoint must use https scheme",
	)
	ErrAccountQuotaMonitorEndpointPrivate = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_ENDPOINT_PRIVATE", "endpoint must be a public host",
	)
	ErrAccountQuotaMonitorEndpointUnreachable = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_ENDPOINT_UNREACHABLE", "endpoint hostname could not be resolved",
	)
	ErrAccountQuotaMonitorInvalidThreshold = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_INVALID_THRESHOLD", "low_balance_threshold must be >= 0",
	)
	ErrAccountQuotaMonitorKeyDecryptFailed = infraerrors.InternalServer(
		"ACCOUNT_QUOTA_MONITOR_KEY_DECRYPT_FAILED", "quota monitor api key override decryption failed; please re-edit the monitor with a fresh key",
	)
	ErrAccountQuotaMonitorFetchFailed = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_FETCH_FAILED", "quota fetch failed; monitor was not saved",
	)
	ErrAccountQuotaMonitorBatchNoAccounts = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_BATCH_NO_ACCOUNTS", "no accounts matched quota monitor batch request",
	)
	ErrAccountQuotaMonitorBatchTooLarge = infraerrors.BadRequest(
		"ACCOUNT_QUOTA_MONITOR_BATCH_TOO_LARGE", "too many accounts matched quota monitor batch request",
	)
)

// AccountQuotaMonitor describes an upstream account balance/quota monitor.
// APIKeyOverride is plaintext only inside service methods after decryption; handlers must never serialize it.
type AccountQuotaMonitor struct {
	ID                  int64
	Name                string
	AccountID           int64
	AccountName         string
	AccountPlatform     string
	AccountType         string
	Provider            string
	Endpoint            string
	APIKeyOverride      string
	Enabled             bool
	IntervalSeconds     int
	LowBalanceThreshold *float64
	Currency            string
	LastBalance         *float64
	LastQuotaTotal      *float64
	LastQuotaUsed       *float64
	LastCheckedAt       *time.Time
	LastStatus          string
	LastMessage         string
	CreatedBy           int64
	CreatedAt           time.Time
	UpdatedAt           time.Time

	APIKeyOverrideSet           bool
	APIKeyOverrideDecryptFailed bool

	Metrics AccountQuotaMonitorMetrics
}

type AccountQuotaMonitorMetrics struct {
	Consumption24h         *float64
	AvgDailyConsumption    *float64
	EstimatedDaysRemaining *float64
	EstimatedDepletedAt    *time.Time
}

type AccountQuotaMonitorAccountFilters struct {
	Platform    string
	AccountType string
	Status      string
	Search      string
	GroupID     int64
	PrivacyMode string
}

type AccountQuotaMonitorBatchCreateParams struct {
	AccountIDs          []int64
	Filters             AccountQuotaMonitorAccountFilters
	Provider            string
	Endpoint            string
	APIKeyOverride      string
	Enabled             bool
	IntervalSeconds     int
	LowBalanceThreshold *float64
	Currency            string
	CreatedBy           int64
	UpdateExisting      bool
	MaxAccounts         int
}

type AccountQuotaMonitorBatchFailure struct {
	AccountID   int64  `json:"account_id"`
	AccountName string `json:"account_name"`
	Reason      string `json:"reason"`
}

type AccountQuotaMonitorBatchCreateResult struct {
	Selected           int64                             `json:"selected"`
	Created            int64                             `json:"created"`
	Updated            int64                             `json:"updated"`
	SkippedExisting    int64                             `json:"skipped_existing"`
	SkippedDuplicate   int64                             `json:"skipped_duplicate"`
	DuplicateEndpoints []string                          `json:"duplicate_endpoints"`
	Failed             int64                             `json:"failed"`
	Items              []*AccountQuotaMonitor            `json:"-"`
	Failures           []AccountQuotaMonitorBatchFailure `json:"failures"`
}

type AccountQuotaMonitorListParams struct {
	Page      int
	PageSize  int
	Provider  string
	Enabled   *bool
	AccountID int64
	Search    string
}

type AccountQuotaMonitorCreateParams struct {
	Name                string
	AccountID           int64
	Provider            string
	Endpoint            string
	APIKeyOverride      string
	Enabled             bool
	IntervalSeconds     int
	LowBalanceThreshold *float64
	Currency            string
	CreatedBy           int64
}

type AccountQuotaMonitorUpdateParams struct {
	Name                     *string
	AccountID                *int64
	Provider                 *string
	Endpoint                 *string
	APIKeyOverride           *string
	ClearAPIKeyOverride      bool
	Enabled                  *bool
	IntervalSeconds          *int
	LowBalanceThreshold      *float64
	ClearLowBalanceThreshold bool
	Currency                 *string
}

type AccountQuotaCheckResult struct {
	MonitorID  int64
	AccountID  int64
	Balance    *float64
	QuotaTotal *float64
	QuotaUsed  *float64
	Currency   string
	Status     string
	Message    string
	CheckedAt  time.Time
}

type AccountQuotaMonitorHistoryEntry struct {
	ID         int64
	MonitorID  int64
	AccountID  int64
	Balance    *float64
	QuotaTotal *float64
	QuotaUsed  *float64
	Currency   string
	Status     string
	Message    string
	CheckedAt  time.Time
}

type AccountQuotaCurrencyTotal struct {
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
	Count    int64   `json:"count"`
}

type AccountQuotaMonitorSummary struct {
	Total                  int64                       `json:"total"`
	Enabled                int64                       `json:"enabled"`
	OK                     int64                       `json:"ok"`
	LowBalance             int64                       `json:"low_balance"`
	Error                  int64                       `json:"error"`
	Unknown                int64                       `json:"unknown"`
	TotalBalanceByCurrency []AccountQuotaCurrencyTotal `json:"total_balance_by_currency"`
}

type AccountQuotaTrendPoint struct {
	Bucket   time.Time `json:"bucket"`
	Currency string    `json:"currency"`
	Balance  float64   `json:"balance"`
	Count    int64     `json:"count"`
}
