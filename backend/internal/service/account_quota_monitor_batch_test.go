package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type quotaBatchEncryptorStub struct{}

func (quotaBatchEncryptorStub) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}
func (quotaBatchEncryptorStub) Decrypt(ciphertext string) (string, error) {
	return strings.TrimPrefix(ciphertext, "enc:"), nil
}

type quotaBatchFetchStub struct {
	errByAccount map[int64]error
	calls        []accountQuotaFetchInput
}

func (f *quotaBatchFetchStub) Fetch(_ context.Context, in accountQuotaFetchInput) (*AccountQuotaCheckResult, error) {
	f.calls = append(f.calls, in)
	if f.errByAccount != nil {
		if err := f.errByAccount[quotaFetchAccountIDFromKey(in.APIKey)]; err != nil {
			return nil, err
		}
	}
	balance := 1.0
	return &AccountQuotaCheckResult{Balance: &balance, Currency: "USD", Status: QuotaMonitorStatusOK}, nil
}

func quotaFetchAccountIDFromKey(key string) int64 {
	var id int64
	_, _ = fmt.Sscanf(key, "key-%d", &id)
	return id
}

func newQuotaBatchService(repo *quotaBatchRepoStub, accountRepo *quotaBatchAccountRepoStub) (*AccountQuotaMonitorService, *quotaBatchFetchStub) {
	svc := NewAccountQuotaMonitorService(repo, accountRepo, quotaBatchEncryptorStub{})
	fetcher := &quotaBatchFetchStub{}
	svc.fetcher = fetcher
	return svc, fetcher
}

type quotaBatchRepoStub struct {
	created  []*AccountQuotaMonitor
	updated  []*AccountQuotaMonitor
	existing map[int64]*AccountQuotaMonitor
}

func (r *quotaBatchRepoStub) Create(_ context.Context, m *AccountQuotaMonitor) error {
	cp := *m
	cp.ID = int64(100 + len(r.created))
	*m = cp
	r.created = append(r.created, &cp)
	return nil
}
func (r *quotaBatchRepoStub) GetByID(context.Context, int64) (*AccountQuotaMonitor, error) {
	return nil, ErrAccountQuotaMonitorNotFound
}
func (r *quotaBatchRepoStub) Update(_ context.Context, m *AccountQuotaMonitor) error {
	cp := *m
	r.updated = append(r.updated, &cp)
	return nil
}
func (r *quotaBatchRepoStub) Delete(context.Context, int64) error { return nil }
func (r *quotaBatchRepoStub) List(context.Context, AccountQuotaMonitorListParams) ([]*AccountQuotaMonitor, int64, error) {
	out := make([]*AccountQuotaMonitor, 0, len(r.existing))
	for _, item := range r.existing {
		out = append(out, item)
	}
	return out, int64(len(out)), nil
}
func (r *quotaBatchRepoStub) ListEnabled(context.Context) ([]*AccountQuotaMonitor, error) {
	return nil, nil
}
func (r *quotaBatchRepoStub) PersistCheckResult(context.Context, *AccountQuotaCheckResult) error {
	return nil
}
func (r *quotaBatchRepoStub) ListHistory(context.Context, int64, int) ([]*AccountQuotaMonitorHistoryEntry, error) {
	return nil, nil
}
func (r *quotaBatchRepoStub) ComputeMetricsFor(_ context.Context, ids []int64) (map[int64]AccountQuotaMonitorMetrics, error) {
	out := make(map[int64]AccountQuotaMonitorMetrics, len(ids))
	for _, id := range ids {
		out[id] = AccountQuotaMonitorMetrics{}
	}
	return out, nil
}
func (r *quotaBatchRepoStub) Summary(context.Context) (*AccountQuotaMonitorSummary, error) {
	return &AccountQuotaMonitorSummary{}, nil
}
func (r *quotaBatchRepoStub) Trend(context.Context, time.Time, string) ([]*AccountQuotaTrendPoint, error) {
	return nil, nil
}
func (r *quotaBatchRepoStub) FindByAccountIDs(_ context.Context, accountIDs []int64) (map[int64]*AccountQuotaMonitor, error) {
	out := make(map[int64]*AccountQuotaMonitor, len(accountIDs))
	for _, id := range accountIDs {
		if r.existing != nil && r.existing[id] != nil {
			out[id] = r.existing[id]
		}
	}
	return out, nil
}

type quotaBatchAccountRepoStub struct {
	accounts []Account
	filters  AccountQuotaMonitorAccountFilters
}

func (r *quotaBatchAccountRepoStub) Create(context.Context, *Account) error { return nil }
func (r *quotaBatchAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			return &r.accounts[i], nil
		}
	}
	return nil, ErrAccountNotFound
}
func (r *quotaBatchAccountRepoStub) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	want := map[int64]struct{}{}
	for _, id := range ids {
		want[id] = struct{}{}
	}
	for i := range r.accounts {
		if _, ok := want[r.accounts[i].ID]; ok {
			out = append(out, &r.accounts[i])
		}
	}
	return out, nil
}
func (r *quotaBatchAccountRepoStub) ExistsByID(context.Context, int64) (bool, error) {
	return true, nil
}
func (r *quotaBatchAccountRepoStub) GetByCRSAccountID(context.Context, string) (*Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) FindByExtraField(context.Context, string, any) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) Update(context.Context, *Account) error { return nil }
func (r *quotaBatchAccountRepoStub) Delete(context.Context, int64) error    { return nil }
func (r *quotaBatchAccountRepoStub) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return r.accounts, &pagination.PaginationResult{Total: int64(len(r.accounts)), Page: 1, PageSize: len(r.accounts), Pages: 1}, nil
}
func (r *quotaBatchAccountRepoStub) ListWithFilters(_ context.Context, _ pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	r.filters = AccountQuotaMonitorAccountFilters{Platform: platform, AccountType: accountType, Status: status, Search: search, GroupID: groupID, PrivacyMode: privacyMode}
	return r.accounts, &pagination.PaginationResult{Total: int64(len(r.accounts)), Page: 1, PageSize: len(r.accounts), Pages: 1}, nil
}
func (r *quotaBatchAccountRepoStub) ListByGroup(context.Context, int64) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListActive(context.Context) ([]Account, error) { return nil, nil }
func (r *quotaBatchAccountRepoStub) ListOAuthRefreshCandidates(context.Context) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) UpdateLastUsed(context.Context, int64) error { return nil }
func (r *quotaBatchAccountRepoStub) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) SetError(context.Context, int64, string) error { return nil }
func (r *quotaBatchAccountRepoStub) ClearError(context.Context, int64) error       { return nil }
func (r *quotaBatchAccountRepoStub) SetSchedulable(context.Context, int64, bool) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *quotaBatchAccountRepoStub) BindGroups(context.Context, int64, []int64) error { return nil }
func (r *quotaBatchAccountRepoStub) ListSchedulable(context.Context) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListSchedulableByPlatforms(context.Context, []string) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	return nil, nil
}
func (r *quotaBatchAccountRepoStub) SetRateLimited(context.Context, int64, time.Time) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time, ...string) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) SetOverloaded(context.Context, int64, time.Time) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) ClearTempUnschedulable(context.Context, int64) error { return nil }
func (r *quotaBatchAccountRepoStub) ClearRateLimit(context.Context, int64) error         { return nil }
func (r *quotaBatchAccountRepoStub) ClearAntigravityQuotaScopes(context.Context, int64) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) ClearModelRateLimits(context.Context, int64) error { return nil }
func (r *quotaBatchAccountRepoStub) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) UpdateSessionWindowEnd(context.Context, int64, time.Time) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) UpdateExtra(context.Context, int64, map[string]any) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) BulkUpdate(context.Context, []int64, AccountBulkUpdate) (int64, error) {
	return 0, nil
}
func (r *quotaBatchAccountRepoStub) IncrementQuotaUsed(context.Context, int64, float64) error {
	return nil
}
func (r *quotaBatchAccountRepoStub) ResetQuotaUsed(context.Context, int64) error { return nil }
func (r *quotaBatchAccountRepoStub) RevertProxyFallback(context.Context, int64) error {
	return nil
}

func TestAccountQuotaMonitorBatchCreateCreatesAllMatchedAccounts(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{
		{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay1.example.com", "api_key": "key-1"}},
		{ID: 2, Name: "a2", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay2.example.com", "api_key": "key-2"}},
	}}
	repo := &quotaBatchRepoStub{}
	svc, _ := newQuotaBatchService(repo, accountRepo)

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		Filters:         AccountQuotaMonitorAccountFilters{Platform: PlatformOpenAI, AccountType: AccountTypeAPIKey, Status: StatusActive, Search: "a"},
		Provider:        QuotaMonitorProviderSub2API,
		Enabled:         true,
		IntervalSeconds: 3600,
		Currency:        "usd",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), res.Selected)
	require.Equal(t, int64(2), res.Created)
	require.Len(t, repo.created, 2)
	require.Equal(t, PlatformOpenAI, accountRepo.filters.Platform)
	require.Equal(t, AccountTypeAPIKey, accountRepo.filters.AccountType)
	require.Equal(t, StatusActive, accountRepo.filters.Status)
	require.Equal(t, "a", accountRepo.filters.Search)
	require.Equal(t, "USD", repo.created[0].Currency)
}

func TestAccountQuotaMonitorCandidateOverviewGroupsByEndpointAndCoverage(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{
		{ID: 1, Name: "pool-a", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Credentials: map[string]any{"base_url": "https://ai.jgy.ai/v1", "api_key": "key-1"}},
		{ID: 2, Name: "pool-b", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Credentials: map[string]any{"base_url": "https://ai.jgy.ai/api/v1/", "api_key": "key-2"}},
		{ID: 3, Name: "relay-c", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Credentials: map[string]any{"base_url": "https://relay.example.com", "api_key": "key-3"}},
		{ID: 4, Name: "no-url", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Credentials: map[string]any{"api_key": "key-4"}},
	}}
	repo := &quotaBatchRepoStub{existing: map[int64]*AccountQuotaMonitor{
		1: {ID: 9, AccountID: 1, Name: "existing", Provider: QuotaMonitorProviderSub2API, Endpoint: "https://ai.jgy.ai", Enabled: true, IntervalSeconds: 3600, Currency: "USD"},
	}}
	svc, _ := newQuotaBatchService(repo, accountRepo)

	got, err := svc.CandidateOverview(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(4), got.TotalAccounts)
	require.Equal(t, int64(3), got.AccountsWithEndpoint)
	require.Equal(t, int64(1), got.AccountsWithoutEndpoint)
	require.Equal(t, int64(2), got.EndpointGroups)
	require.Equal(t, int64(1), got.CoveredGroups)
	require.Equal(t, int64(1), got.MissingGroups)
	require.Equal(t, QuotaMonitorProviderSub2API, got.Groups[1].Provider)
	require.True(t, got.Groups[1].Covered)
	require.Equal(t, int64(2), got.Groups[1].AccountCount)
	require.Equal(t, int64(0), got.Groups[1].MissingAccountCount)
	require.Equal(t, int64(3), got.Groups[0].RepresentativeAccountID)
	require.Equal(t, QuotaMonitorProviderCustom, got.Groups[0].Provider)
	require.False(t, got.Groups[0].ProviderDetected)
}

func TestAccountQuotaMonitorBatchCreateSkipsExistingByDefault(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay1.example.com", "api_key": "key-1"}}}}
	repo := &quotaBatchRepoStub{existing: map[int64]*AccountQuotaMonitor{1: {ID: 7, AccountID: 1, Name: "old", Provider: QuotaMonitorProviderNewAPI, Enabled: true, IntervalSeconds: 3600, Currency: "USD"}}}
	svc, _ := newQuotaBatchService(repo, accountRepo)

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		AccountIDs:          []int64{1},
		Provider:            QuotaMonitorProviderSub2API,
		Enabled:             true,
		IntervalSeconds:     3600,
		Currency:            "USD",
		UpdateExisting:      false,
		LowBalanceThreshold: nil,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.SkippedExisting)
	require.Empty(t, repo.created)
	require.Empty(t, repo.updated)
}

func TestAccountQuotaMonitorBatchCreateCanUpdateExisting(t *testing.T) {
	threshold := 10.0
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay1.example.com", "api_key": "key-1"}}}}
	repo := &quotaBatchRepoStub{existing: map[int64]*AccountQuotaMonitor{1: {ID: 7, AccountID: 1, Name: "old", Provider: QuotaMonitorProviderNewAPI, Enabled: true, IntervalSeconds: 3600, Currency: "USD"}}}
	svc, _ := newQuotaBatchService(repo, accountRepo)

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		AccountIDs:          []int64{1},
		Provider:            QuotaMonitorProviderSub2API,
		Endpoint:            "",
		Enabled:             false,
		IntervalSeconds:     7200,
		LowBalanceThreshold: &threshold,
		Currency:            "cny",
		UpdateExisting:      true,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.Updated)
	require.Len(t, repo.updated, 1)
	updated := repo.updated[0]
	require.Equal(t, "a1 额度监控", updated.Name)
	require.Equal(t, QuotaMonitorProviderSub2API, updated.Provider)
	require.False(t, updated.Enabled)
	require.Equal(t, 7200, updated.IntervalSeconds)
	require.Equal(t, "CNY", updated.Currency)
	require.NotNil(t, updated.LowBalanceThreshold)
	require.Equal(t, threshold, *updated.LowBalanceThreshold)
}

func TestAccountQuotaMonitorBatchUpdateExistingSkipsFetchFailure(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay1.example.com", "api_key": "key-1"}}}}
	repo := &quotaBatchRepoStub{existing: map[int64]*AccountQuotaMonitor{1: {ID: 7, AccountID: 1, Name: "old", Provider: QuotaMonitorProviderNewAPI, Enabled: true, IntervalSeconds: 3600, Currency: "USD", APIKeyOverride: "enc:key-2", APIKeyOverrideSet: true}}}
	svc, fetcher := newQuotaBatchService(repo, accountRepo)
	fetcher.errByAccount = map[int64]error{2: fmt.Errorf("fetch failed")}

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		AccountIDs:      []int64{1},
		Provider:        QuotaMonitorProviderSub2API,
		Enabled:         true,
		IntervalSeconds: 7200,
		Currency:        "USD",
		UpdateExisting:  true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(0), res.Updated)
	require.Equal(t, int64(1), res.Failed)
	require.Empty(t, repo.updated)
	require.Len(t, res.Failures, 1)
	require.Equal(t, int64(1), res.Failures[0].AccountID)
}

func TestAccountQuotaMonitorBatchUpdateExistingUsesPlainOverrideAndStoresEncrypted(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay1.example.com", "api_key": "key-1"}}}}
	repo := &quotaBatchRepoStub{existing: map[int64]*AccountQuotaMonitor{1: {ID: 7, AccountID: 1, Name: "old", Provider: QuotaMonitorProviderNewAPI, Enabled: true, IntervalSeconds: 3600, Currency: "USD", APIKeyOverride: "enc:key-1", APIKeyOverrideSet: true}}}
	svc, fetcher := newQuotaBatchService(repo, accountRepo)

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		AccountIDs:      []int64{1},
		Provider:        QuotaMonitorProviderSub2API,
		APIKeyOverride:  "key-9",
		Enabled:         true,
		IntervalSeconds: 7200,
		Currency:        "USD",
		UpdateExisting:  true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), res.Updated)
	require.Len(t, fetcher.calls, 1)
	require.Equal(t, "key-9", fetcher.calls[0].APIKey)
	require.Len(t, repo.updated, 1)
	require.Equal(t, "enc:key-9", repo.updated[0].APIKeyOverride)
	require.True(t, repo.updated[0].APIKeyOverrideSet)
}

func TestAccountQuotaMonitorBatchCreateDeduplicatesByEffectiveEndpoint(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{
		{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay.example.com/v1/", "api_key": "key-1"}},
		{ID: 2, Name: "a2", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://RELAY.example.com/api/v1", "api_key": "key-2"}},
		{ID: 3, Name: "a3", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://other.example.com", "api_key": "key-3"}},
	}}
	repo := &quotaBatchRepoStub{}
	svc, _ := newQuotaBatchService(repo, accountRepo)

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		Provider:        QuotaMonitorProviderSub2API,
		Enabled:         true,
		IntervalSeconds: 3600,
		Currency:        "USD",
	})

	require.NoError(t, err)
	require.Equal(t, int64(3), res.Selected)
	require.Equal(t, int64(2), res.Created)
	require.Equal(t, int64(1), res.SkippedDuplicate)
	require.ElementsMatch(t, []string{"https://relay.example.com"}, res.DuplicateEndpoints)
	require.Len(t, repo.created, 2)
	require.Equal(t, int64(1), repo.created[0].AccountID)
	require.Equal(t, int64(3), repo.created[1].AccountID)
}

func TestAccountQuotaMonitorBatchCreateDedupPrefersExistingEndpointOwner(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{
		{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://same.example.com/v1", "api_key": "key-1"}},
		{ID: 2, Name: "a2", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://same.example.com/api/v1/", "api_key": "key-2"}},
	}}
	repo := &quotaBatchRepoStub{existing: map[int64]*AccountQuotaMonitor{
		2: {ID: 9, AccountID: 2, Name: "existing", Provider: QuotaMonitorProviderSub2API, Enabled: true, IntervalSeconds: 3600, Currency: "USD"},
	}}
	svc, _ := newQuotaBatchService(repo, accountRepo)

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		Provider:        QuotaMonitorProviderSub2API,
		Enabled:         true,
		IntervalSeconds: 3600,
		Currency:        "USD",
		UpdateExisting:  false,
	})

	require.NoError(t, err)
	require.Equal(t, int64(0), res.Created)
	require.Equal(t, int64(1), res.SkippedExisting)
	require.Equal(t, int64(1), res.SkippedDuplicate)
	require.Empty(t, repo.created)
	require.Empty(t, repo.updated)
}

func TestAccountQuotaMonitorBatchCreateSkipsFetchFailures(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{
		{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://ok.example.com", "api_key": "key-1"}},
		{ID: 2, Name: "a2", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://bad.example.com", "api_key": "key-2"}},
	}}
	repo := &quotaBatchRepoStub{}
	svc, fetcher := newQuotaBatchService(repo, accountRepo)
	fetcher.errByAccount = map[int64]error{2: fmt.Errorf("fetch failed")}

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		Provider:        QuotaMonitorProviderSub2API,
		Enabled:         true,
		IntervalSeconds: 3600,
		Currency:        "USD",
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), res.Created)
	require.Equal(t, int64(1), res.Failed)
	require.Len(t, repo.created, 1)
	require.Equal(t, int64(1), repo.created[0].AccountID)
	require.Len(t, res.Failures, 1)
	require.Equal(t, int64(2), res.Failures[0].AccountID)
}

func TestNormalizeQuotaMonitorEndpointKey(t *testing.T) {
	require.Equal(t, "https://relay.example.com", normalizeQuotaMonitorEndpointKey("https://RELAY.example.com/v1/?x=1#frag"))
	require.Equal(t, "https://relay.example.com", normalizeQuotaMonitorEndpointKey("https://relay.example.com/api/v1/"))
	require.Equal(t, "https://relay.example.com/custom", normalizeQuotaMonitorEndpointKey("https://relay.example.com/custom/"))
}

func TestAccountQuotaMonitorBatchCreateReactivatesDisabledExistingByDefault(t *testing.T) {
	accountRepo := &quotaBatchAccountRepoStub{accounts: []Account{{ID: 1, Name: "a1", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay1.example.com", "api_key": "key-1"}}}}
	repo := &quotaBatchRepoStub{existing: map[int64]*AccountQuotaMonitor{1: {ID: 7, AccountID: 1, Name: "old", Provider: QuotaMonitorProviderCustom, Enabled: false, IntervalSeconds: 3600, Currency: "USD"}}}
	svc, _ := newQuotaBatchService(repo, accountRepo)

	res, err := svc.BatchCreate(context.Background(), AccountQuotaMonitorBatchCreateParams{
		AccountIDs:      []int64{1},
		Provider:        QuotaMonitorProviderSub2API,
		Enabled:         true,
		IntervalSeconds: 600,
		Currency:        "USD",
		UpdateExisting:  false,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.Updated)
	require.Zero(t, res.SkippedExisting)
	require.Len(t, repo.updated, 1)
	require.True(t, repo.updated[0].Enabled)
	require.Equal(t, QuotaMonitorProviderSub2API, repo.updated[0].Provider)
	require.Equal(t, 600, repo.updated[0].IntervalSeconds)
}
