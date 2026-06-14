package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// AccountQuotaMonitorRepository is the persistence port for upstream account quota monitors.
type AccountQuotaMonitorRepository interface {
	Create(ctx context.Context, m *AccountQuotaMonitor) error
	GetByID(ctx context.Context, id int64) (*AccountQuotaMonitor, error)
	Update(ctx context.Context, m *AccountQuotaMonitor) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, params AccountQuotaMonitorListParams) ([]*AccountQuotaMonitor, int64, error)
	ListEnabled(ctx context.Context) ([]*AccountQuotaMonitor, error)
	PersistCheckResult(ctx context.Context, result *AccountQuotaCheckResult) error
	ListHistory(ctx context.Context, monitorID int64, limit int) ([]*AccountQuotaMonitorHistoryEntry, error)
	ComputeMetricsFor(ctx context.Context, ids []int64) (map[int64]AccountQuotaMonitorMetrics, error)
	Summary(ctx context.Context) (*AccountQuotaMonitorSummary, error)
	Trend(ctx context.Context, since time.Time, bucket string) ([]*AccountQuotaTrendPoint, error)
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*AccountQuotaMonitor, error)
}

// AccountQuotaMonitorScheduler lets CRUD operations keep the background runner in sync.
type AccountQuotaMonitorScheduler interface {
	Schedule(m *AccountQuotaMonitor)
	Unschedule(id int64)
}

type AccountQuotaMonitorService struct {
	repo        AccountQuotaMonitorRepository
	accountRepo AccountRepository
	encryptor   SecretEncryptor
	fetcher     accountQuotaFetchClient
	scheduler   AccountQuotaMonitorScheduler
}

func NewAccountQuotaMonitorService(repo AccountQuotaMonitorRepository, accountRepo AccountRepository, encryptor SecretEncryptor) *AccountQuotaMonitorService {
	return &AccountQuotaMonitorService{
		repo:        repo,
		accountRepo: accountRepo,
		encryptor:   encryptor,
		fetcher:     newAccountQuotaFetcher(),
	}
}

func (s *AccountQuotaMonitorService) SetScheduler(scheduler AccountQuotaMonitorScheduler) {
	s.scheduler = scheduler
}

func (s *AccountQuotaMonitorService) List(ctx context.Context, params AccountQuotaMonitorListParams) ([]*AccountQuotaMonitor, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 200 {
		params.PageSize = 20
	}
	params.Provider = normalizeQuotaMonitorProvider(params.Provider)
	if params.Provider != "" {
		if err := validateQuotaMonitorProvider(params.Provider); err != nil {
			return nil, 0, err
		}
	}
	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	s.decryptOverridesInPlace(items)
	s.enrichMetrics(ctx, items)
	return items, total, nil
}

func (s *AccountQuotaMonitorService) Get(ctx context.Context, id int64) (*AccountQuotaMonitor, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.decryptOverrideInPlace(m)
	s.enrichMetrics(ctx, []*AccountQuotaMonitor{m})
	return m, nil
}

func (s *AccountQuotaMonitorService) Create(ctx context.Context, p AccountQuotaMonitorCreateParams) (*AccountQuotaMonitor, error) {
	if err := s.validateCreate(ctx, p); err != nil {
		return nil, err
	}
	account, err := s.accountRepo.GetByID(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}
	m := &AccountQuotaMonitor{
		Name:                strings.TrimSpace(p.Name),
		AccountID:           p.AccountID,
		Provider:            normalizeQuotaMonitorProvider(p.Provider),
		Endpoint:            normalizeQuotaEndpoint(p.Endpoint),
		Enabled:             p.Enabled,
		IntervalSeconds:     normalizeQuotaInterval(p.IntervalSeconds),
		LowBalanceThreshold: cloneFloat64Ptr(p.LowBalanceThreshold),
		Currency:            normalizeQuotaCurrency(p.Currency),
		LastStatus:          QuotaMonitorStatusUnknown,
		CreatedBy:           p.CreatedBy,
	}
	if err := s.validateMonitorFetch(ctx, m, account, strings.TrimSpace(p.APIKeyOverride), true); err != nil {
		return nil, err
	}
	encrypted, err := s.encryptOverride(p.APIKeyOverride)
	if err != nil {
		return nil, err
	}
	m.APIKeyOverride = encrypted
	m.APIKeyOverrideSet = strings.TrimSpace(encrypted) != ""
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	m.APIKeyOverride = strings.TrimSpace(p.APIKeyOverride)
	m.APIKeyOverrideSet = m.APIKeyOverride != ""
	if s.scheduler != nil {
		s.scheduler.Schedule(m)
	}
	return m, nil
}

func (s *AccountQuotaMonitorService) BatchCreate(ctx context.Context, p AccountQuotaMonitorBatchCreateParams) (*AccountQuotaMonitorBatchCreateResult, error) {
	if err := s.validateBatchCreate(ctx, p); err != nil {
		return nil, err
	}
	accounts, err := s.resolveBatchAccounts(ctx, p)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrAccountQuotaMonitorBatchNoAccounts
	}
	maxAccounts := p.MaxAccounts
	if maxAccounts <= 0 {
		maxAccounts = quotaMonitorBatchDefaultMaxAccounts
	}
	if len(accounts) > maxAccounts {
		return nil, ErrAccountQuotaMonitorBatchTooLarge
	}
	existingByAccount, err := s.repo.FindByAccountIDs(ctx, accountIDsFromAccounts(accounts))
	if err != nil {
		return nil, fmt.Errorf("find existing account quota monitors: %w", err)
	}

	result := &AccountQuotaMonitorBatchCreateResult{Selected: int64(len(accounts))}
	endpointOwners := quotaMonitorEndpointOwners(accounts, existingByAccount, p.Endpoint)
	for i := range accounts {
		account := &accounts[i]
		endpointKey := quotaMonitorEndpointKeyForAccount(account, p.Endpoint)
		if endpointKey != "" {
			if ownerID, ok := endpointOwners[endpointKey]; ok && ownerID != account.ID {
				result.SkippedDuplicate++
				result.DuplicateEndpoints = appendUniqueQuotaEndpoint(result.DuplicateEndpoints, endpointKey)
				continue
			}
		}
		if existing := existingByAccount[account.ID]; existing != nil {
			if !p.UpdateExisting {
				result.SkippedExisting++
				continue
			}
			if err := s.applyBatchUpdateToExisting(ctx, existing, account, p); err != nil {
				result.Failed++
				result.Failures = append(result.Failures, AccountQuotaMonitorBatchFailure{AccountID: account.ID, AccountName: account.Name, Reason: err.Error()})
				continue
			}
			if err := s.repo.Update(ctx, existing); err != nil {
				result.Failed++
				result.Failures = append(result.Failures, AccountQuotaMonitorBatchFailure{AccountID: account.ID, AccountName: account.Name, Reason: err.Error()})
				continue
			}
			s.decryptOverrideInPlace(existing)
			if s.scheduler != nil {
				s.scheduler.Schedule(existing)
			}
			result.Updated++
			result.Items = append(result.Items, existing)
			continue
		}

		m, err := s.createMonitorForAccount(ctx, account, p)
		if err != nil {
			result.Failed++
			result.Failures = append(result.Failures, AccountQuotaMonitorBatchFailure{AccountID: account.ID, AccountName: account.Name, Reason: err.Error()})
			continue
		}
		result.Created++
		result.Items = append(result.Items, m)
	}
	return result, nil
}

func (s *AccountQuotaMonitorService) Update(ctx context.Context, id int64, p AccountQuotaMonitorUpdateParams) (*AccountQuotaMonitor, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.applyUpdate(ctx, existing, p); err != nil {
		return nil, err
	}
	plainOverride, overrideUpdated, err := s.applyOverrideUpdate(existing, p)
	if err != nil {
		return nil, err
	}
	account, err := s.accountRepo.GetByID(ctx, existing.AccountID)
	if err != nil {
		return nil, err
	}
	if err := s.validateMonitorFetch(ctx, existing, account, plainOverride, overrideUpdated); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	if overrideUpdated {
		existing.APIKeyOverride = plainOverride
		existing.APIKeyOverrideSet = strings.TrimSpace(plainOverride) != ""
	} else {
		s.decryptOverrideInPlace(existing)
	}
	if s.scheduler != nil {
		s.scheduler.Schedule(existing)
	}
	return existing, nil
}

func (s *AccountQuotaMonitorService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.scheduler != nil {
		s.scheduler.Unschedule(id)
	}
	return nil
}

func (s *AccountQuotaMonitorService) RunCheck(ctx context.Context, id int64) (*AccountQuotaCheckResult, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.APIKeyOverrideDecryptFailed {
		return nil, ErrAccountQuotaMonitorKeyDecryptFailed
	}
	result := s.runCheckForMonitor(ctx, m)
	if err := s.repo.PersistCheckResult(ctx, result); err != nil {
		slog.Error("account_quota_monitor: persist check result failed", "monitor_id", m.ID, "error", err)
	}
	return result, nil
}

func (s *AccountQuotaMonitorService) ListHistory(ctx context.Context, id int64, limit int) ([]*AccountQuotaMonitorHistoryEntry, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = QuotaMonitorHistoryDefaultLimit
	}
	if limit > QuotaMonitorHistoryMaxLimit {
		limit = QuotaMonitorHistoryMaxLimit
	}
	return s.repo.ListHistory(ctx, id, limit)
}

func (s *AccountQuotaMonitorService) ListEnabledMonitors(ctx context.Context) ([]*AccountQuotaMonitor, error) {
	items, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	s.decryptOverridesInPlace(items)
	return items, nil
}

func (s *AccountQuotaMonitorService) Summary(ctx context.Context) (*AccountQuotaMonitorSummary, error) {
	return s.repo.Summary(ctx)
}

func (s *AccountQuotaMonitorService) Trend(ctx context.Context, days int, bucket string) ([]*AccountQuotaTrendPoint, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	return s.repo.Trend(ctx, time.Now().UTC().AddDate(0, 0, -days), bucket)
}

func (s *AccountQuotaMonitorService) CandidateOverview(ctx context.Context) (*AccountQuotaMonitorCandidateOverview, error) {
	accounts, err := s.resolveBatchAccounts(ctx, AccountQuotaMonitorBatchCreateParams{
		Filters:     AccountQuotaMonitorAccountFilters{Status: StatusActive},
		MaxAccounts: quotaMonitorBatchDefaultMaxAccounts,
	})
	if err != nil {
		return nil, err
	}

	accountByID := make(map[int64]*Account, len(accounts))
	groupsByEndpoint := make(map[string]*AccountQuotaMonitorCandidateGroup)
	overview := &AccountQuotaMonitorCandidateOverview{TotalAccounts: int64(len(accounts))}

	for i := range accounts {
		account := &accounts[i]
		if account.ID <= 0 {
			continue
		}
		accountByID[account.ID] = account
		endpoint := effectiveQuotaMonitorEndpointForAccount(account, "")
		endpointKey := normalizeQuotaMonitorEndpointKey(endpoint)
		if endpointKey == "" {
			overview.AccountsWithoutEndpoint++
			continue
		}
		overview.AccountsWithEndpoint++
		group := groupsByEndpoint[endpointKey]
		if group == nil {
			provider, detected := detectQuotaMonitorProviderFromEndpoint(endpoint)
			group = &AccountQuotaMonitorCandidateGroup{
				Endpoint:         endpoint,
				EndpointKey:      endpointKey,
				Provider:         provider,
				ProviderDetected: detected,
				StatusCounts:     map[string]int64{},
			}
			groupsByEndpoint[endpointKey] = group
		}
		group.AccountCount++
		group.StatusCounts[account.Status]++
		if group.RepresentativeAccountID == 0 {
			group.RepresentativeAccountID = account.ID
		}
		if len(group.SampleAccounts) < 5 {
			group.SampleAccounts = append(group.SampleAccounts, AccountQuotaMonitorCandidateAccount{
				ID:       account.ID,
				Name:     account.Name,
				Platform: account.Platform,
				Type:     account.Type,
				Status:   account.Status,
			})
		}
	}

	monitors, _, err := s.repo.List(ctx, AccountQuotaMonitorListParams{Page: 1, PageSize: quotaMonitorBatchDefaultMaxAccounts})
	if err != nil {
		return nil, err
	}
	for _, monitor := range monitors {
		if monitor == nil {
			continue
		}
		account := accountByID[monitor.AccountID]
		endpoint := strings.TrimSpace(monitor.Endpoint)
		if endpoint == "" && account != nil {
			endpoint = effectiveQuotaMonitorEndpointForAccount(account, "")
		}
		endpointKey := normalizeQuotaMonitorEndpointKey(endpoint)
		if endpointKey == "" {
			continue
		}
		group := groupsByEndpoint[endpointKey]
		if group == nil {
			provider := normalizeQuotaMonitorProvider(monitor.Provider)
			detected := provider == QuotaMonitorProviderSub2API || provider == QuotaMonitorProviderNewAPI
			if provider == "" {
				provider, detected = detectQuotaMonitorProviderFromEndpoint(endpoint)
			}
			group = &AccountQuotaMonitorCandidateGroup{
				Endpoint:         endpoint,
				EndpointKey:      endpointKey,
				Provider:         provider,
				ProviderDetected: detected,
				StatusCounts:     map[string]int64{},
			}
			groupsByEndpoint[endpointKey] = group
		}
		group.MonitorCount++
		group.ExistingMonitorIDs = append(group.ExistingMonitorIDs, monitor.ID)
		if group.Provider == "" || group.Provider == QuotaMonitorProviderCustom {
			if normalized := normalizeQuotaMonitorProvider(monitor.Provider); normalized != "" {
				group.Provider = normalized
				group.ProviderDetected = normalized == QuotaMonitorProviderSub2API || normalized == QuotaMonitorProviderNewAPI
			}
		}
	}

	overview.Groups = make([]AccountQuotaMonitorCandidateGroup, 0, len(groupsByEndpoint))
	for _, group := range groupsByEndpoint {
		group.Covered = group.MonitorCount > 0
		if !group.Covered {
			group.MissingAccountCount = group.AccountCount
		}
		if group.MonitorCount > 1 {
			group.DuplicateMonitorCount = group.MonitorCount - 1
			overview.DuplicateMonitorGroups++
		}
		if group.Provider == "" {
			group.Provider = QuotaMonitorProviderCustom
		}
		overview.EndpointGroups++
		if group.Covered {
			overview.CoveredGroups++
		} else {
			overview.MissingGroups++
		}
		if group.ProviderDetected {
			overview.DetectedProviderGroups++
		} else {
			overview.CustomProviderGroups++
		}
		overview.Groups = append(overview.Groups, *group)
	}

	sort.SliceStable(overview.Groups, func(i, j int) bool {
		a, b := overview.Groups[i], overview.Groups[j]
		if a.Covered != b.Covered {
			return !a.Covered
		}
		if a.AccountCount != b.AccountCount {
			return a.AccountCount > b.AccountCount
		}
		return a.EndpointKey < b.EndpointKey
	})
	return overview, nil
}

func (s *AccountQuotaMonitorService) validateBatchCreate(ctx context.Context, p AccountQuotaMonitorBatchCreateParams) error {
	provider := normalizeQuotaMonitorProvider(p.Provider)
	if err := validateQuotaMonitorProvider(provider); err != nil {
		return err
	}
	if err := validateQuotaMonitorInterval(normalizeQuotaInterval(p.IntervalSeconds)); err != nil {
		return err
	}
	if err := validateQuotaEndpoint(p.Endpoint); err != nil {
		return err
	}
	if err := validateQuotaThreshold(p.LowBalanceThreshold); err != nil {
		return err
	}
	if len(p.AccountIDs) == 1 && p.AccountIDs[0] > 0 {
		_, err := s.accountRepo.GetByID(ctx, p.AccountIDs[0])
		return err
	}
	return nil
}

func (s *AccountQuotaMonitorService) resolveBatchAccounts(ctx context.Context, p AccountQuotaMonitorBatchCreateParams) ([]Account, error) {
	if len(p.AccountIDs) > 0 {
		unique := uniquePositiveInt64s(p.AccountIDs)
		if len(unique) == 0 {
			return nil, ErrAccountQuotaMonitorBatchNoAccounts
		}
		accounts, err := s.accountRepo.GetByIDs(ctx, unique)
		if err != nil {
			return nil, err
		}
		out := make([]Account, 0, len(accounts))
		for _, account := range accounts {
			if account != nil {
				out = append(out, *account)
			}
		}
		return out, nil
	}
	params := pagination.PaginationParams{Page: 1, PageSize: quotaMonitorBatchFetchPageSize, SortBy: "id", SortOrder: pagination.SortOrderAsc}
	accounts, pageResult, err := s.accountRepo.ListWithFilters(ctx, params,
		strings.TrimSpace(p.Filters.Platform),
		strings.TrimSpace(p.Filters.AccountType),
		strings.TrimSpace(p.Filters.Status),
		strings.TrimSpace(p.Filters.Search),
		p.Filters.GroupID,
		strings.TrimSpace(p.Filters.PrivacyMode),
	)
	if err != nil {
		return nil, err
	}
	maxAccounts := p.MaxAccounts
	if maxAccounts <= 0 {
		maxAccounts = quotaMonitorBatchDefaultMaxAccounts
	}
	if pageResult != nil && pageResult.Total > int64(maxAccounts) {
		return nil, ErrAccountQuotaMonitorBatchTooLarge
	}
	return accounts, nil
}

func (s *AccountQuotaMonitorService) createMonitorForAccount(ctx context.Context, account *Account, p AccountQuotaMonitorBatchCreateParams) (*AccountQuotaMonitor, error) {
	if account == nil {
		return nil, ErrAccountQuotaMonitorMissingAccount
	}
	endpoint := effectiveQuotaMonitorEndpointForAccount(account, p.Endpoint)
	m := &AccountQuotaMonitor{
		Name:                quotaMonitorNameForAccount(account),
		AccountID:           account.ID,
		AccountName:         account.Name,
		AccountPlatform:     account.Platform,
		AccountType:         account.Type,
		Provider:            normalizeQuotaMonitorProvider(p.Provider),
		Endpoint:            endpoint,
		Enabled:             p.Enabled,
		IntervalSeconds:     normalizeQuotaInterval(p.IntervalSeconds),
		LowBalanceThreshold: cloneFloat64Ptr(p.LowBalanceThreshold),
		Currency:            normalizeQuotaCurrency(p.Currency),
		LastStatus:          QuotaMonitorStatusUnknown,
		CreatedBy:           p.CreatedBy,
	}
	if err := s.validateMonitorFetch(ctx, m, account, strings.TrimSpace(p.APIKeyOverride), true); err != nil {
		return nil, err
	}
	encrypted, err := s.encryptOverride(p.APIKeyOverride)
	if err != nil {
		return nil, err
	}
	m.APIKeyOverride = encrypted
	m.APIKeyOverrideSet = strings.TrimSpace(encrypted) != ""
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	m.APIKeyOverride = strings.TrimSpace(p.APIKeyOverride)
	m.APIKeyOverrideSet = m.APIKeyOverride != ""
	if s.scheduler != nil {
		s.scheduler.Schedule(m)
	}
	return m, nil
}

func (s *AccountQuotaMonitorService) applyBatchUpdateToExisting(ctx context.Context, existing *AccountQuotaMonitor, account *Account, p AccountQuotaMonitorBatchCreateParams) error {
	if existing == nil || account == nil {
		return ErrAccountQuotaMonitorMissingAccount
	}
	endpoint := effectiveQuotaMonitorEndpointForAccount(account, p.Endpoint)
	currency := normalizeQuotaCurrency(p.Currency)
	provider := normalizeQuotaMonitorProvider(p.Provider)
	interval := normalizeQuotaInterval(p.IntervalSeconds)
	plainOverride, err := s.encryptOverride(p.APIKeyOverride)
	if err != nil {
		return err
	}
	existing.Name = quotaMonitorNameForAccount(account)
	existing.AccountID = account.ID
	existing.AccountName = account.Name
	existing.AccountPlatform = account.Platform
	existing.AccountType = account.Type
	existing.Provider = provider
	existing.Endpoint = endpoint
	existing.Enabled = p.Enabled
	existing.IntervalSeconds = interval
	existing.LowBalanceThreshold = cloneFloat64Ptr(p.LowBalanceThreshold)
	existing.Currency = currency
	plainOverrideKnown := false
	validationOverride := ""
	if strings.TrimSpace(p.APIKeyOverride) != "" {
		existing.APIKeyOverride = plainOverride
		existing.APIKeyOverrideSet = true
		plainOverrideKnown = true
		validationOverride = strings.TrimSpace(p.APIKeyOverride)
	}
	if err := s.validateMonitorFetch(ctx, existing, account, validationOverride, plainOverrideKnown); err != nil {
		return err
	}
	return s.applyUpdate(ctx, existing, AccountQuotaMonitorUpdateParams{})
}

func accountIDsFromAccounts(accounts []Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		if accounts[i].ID > 0 {
			ids = append(ids, accounts[i].ID)
		}
	}
	return ids
}

func quotaMonitorEndpointOwners(accounts []Account, existingByAccount map[int64]*AccountQuotaMonitor, overrideEndpoint string) map[string]int64 {
	owners := make(map[string]int64, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		endpointKey := quotaMonitorEndpointKeyForAccount(account, overrideEndpoint)
		if endpointKey == "" || account.ID <= 0 {
			continue
		}
		ownerID, exists := owners[endpointKey]
		if !exists {
			owners[endpointKey] = account.ID
			continue
		}
		if existingByAccount[ownerID] == nil && existingByAccount[account.ID] != nil {
			owners[endpointKey] = account.ID
		}
	}
	return owners
}

func uniquePositiveInt64s(in []int64) []int64 {
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, id := range in {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func quotaMonitorEndpointKeyForAccount(account *Account, overrideEndpoint string) string {
	return normalizeQuotaMonitorEndpointKey(effectiveQuotaMonitorEndpointForAccount(account, overrideEndpoint))
}

func effectiveQuotaMonitorEndpointForAccount(account *Account, overrideEndpoint string) string {
	endpoint := strings.TrimSpace(overrideEndpoint)
	if endpoint == "" && account != nil {
		endpoint = strings.TrimSpace(account.GetCredential("base_url"))
		if endpoint == "" && account.IsCustomBaseURLEnabled() {
			endpoint = strings.TrimSpace(account.GetCustomBaseURL())
		}
	}
	return normalizeQuotaEndpoint(endpoint)
}

func normalizeQuotaMonitorEndpointKey(raw string) string {
	endpoint := normalizeQuotaEndpoint(raw)
	if endpoint == "" {
		return ""
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return strings.ToLower(strings.TrimRight(endpoint, "/"))
	}
	u.Fragment = ""
	u.RawQuery = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	path := strings.TrimRight(u.EscapedPath(), "/")
	if isVersionOnlyPath(path) {
		u.Path = ""
		u.RawPath = ""
	} else {
		u.Path = path
		u.RawPath = ""
	}
	return strings.TrimRight(u.String(), "/")
}

func appendUniqueQuotaEndpoint(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, existing := range items {
		if existing == value {
			return items
		}
	}
	return append(items, value)
}

func quotaMonitorNameForAccount(account *Account) string {
	if account == nil {
		return "账号额度监控"
	}
	name := strings.TrimSpace(account.Name)
	if name == "" {
		name = fmt.Sprintf("账号 #%d", account.ID)
	}
	return name + " 额度监控"
}

func (s *AccountQuotaMonitorService) validateCreate(ctx context.Context, p AccountQuotaMonitorCreateParams) error {
	if strings.TrimSpace(p.Name) == "" {
		return ErrAccountQuotaMonitorMissingName
	}
	if p.AccountID <= 0 {
		return ErrAccountQuotaMonitorMissingAccount
	}
	if _, err := s.accountRepo.GetByID(ctx, p.AccountID); err != nil {
		return err
	}
	provider := normalizeQuotaMonitorProvider(p.Provider)
	if err := validateQuotaMonitorProvider(provider); err != nil {
		return err
	}
	if err := validateQuotaMonitorInterval(normalizeQuotaInterval(p.IntervalSeconds)); err != nil {
		return err
	}
	if err := validateQuotaEndpoint(p.Endpoint); err != nil {
		return err
	}
	return validateQuotaThreshold(p.LowBalanceThreshold)
}

func (s *AccountQuotaMonitorService) applyUpdate(ctx context.Context, existing *AccountQuotaMonitor, p AccountQuotaMonitorUpdateParams) error {
	if p.Name != nil {
		if strings.TrimSpace(*p.Name) == "" {
			return ErrAccountQuotaMonitorMissingName
		}
		existing.Name = strings.TrimSpace(*p.Name)
	}
	if p.AccountID != nil {
		if *p.AccountID <= 0 {
			return ErrAccountQuotaMonitorMissingAccount
		}
		if _, err := s.accountRepo.GetByID(ctx, *p.AccountID); err != nil {
			return err
		}
		existing.AccountID = *p.AccountID
	}
	if p.Provider != nil {
		provider := normalizeQuotaMonitorProvider(*p.Provider)
		if err := validateQuotaMonitorProvider(provider); err != nil {
			return err
		}
		existing.Provider = provider
	}
	if p.Endpoint != nil {
		if err := validateQuotaEndpoint(*p.Endpoint); err != nil {
			return err
		}
		existing.Endpoint = normalizeQuotaEndpoint(*p.Endpoint)
	}
	if p.Enabled != nil {
		existing.Enabled = *p.Enabled
	}
	if p.IntervalSeconds != nil {
		interval := normalizeQuotaInterval(*p.IntervalSeconds)
		if err := validateQuotaMonitorInterval(interval); err != nil {
			return err
		}
		existing.IntervalSeconds = interval
	}
	if p.ClearLowBalanceThreshold {
		existing.LowBalanceThreshold = nil
	} else if p.LowBalanceThreshold != nil {
		if err := validateQuotaThreshold(p.LowBalanceThreshold); err != nil {
			return err
		}
		existing.LowBalanceThreshold = cloneFloat64Ptr(p.LowBalanceThreshold)
	}
	if p.Currency != nil {
		existing.Currency = normalizeQuotaCurrency(*p.Currency)
	}
	return nil
}

func (s *AccountQuotaMonitorService) applyOverrideUpdate(existing *AccountQuotaMonitor, p AccountQuotaMonitorUpdateParams) (plain string, updated bool, err error) {
	if p.ClearAPIKeyOverride {
		existing.APIKeyOverride = ""
		existing.APIKeyOverrideSet = false
		return "", true, nil
	}
	if p.APIKeyOverride == nil || strings.TrimSpace(*p.APIKeyOverride) == "" {
		return "", false, nil
	}
	plain = strings.TrimSpace(*p.APIKeyOverride)
	encrypted, encErr := s.encryptor.Encrypt(plain)
	if encErr != nil {
		return "", false, fmt.Errorf("encrypt quota monitor api key override: %w", encErr)
	}
	existing.APIKeyOverride = encrypted
	existing.APIKeyOverrideSet = true
	return plain, true, nil
}

func (s *AccountQuotaMonitorService) validateMonitorFetch(ctx context.Context, m *AccountQuotaMonitor, account *Account, plainOverride string, overrideKnown bool) error {
	if m == nil || !m.Enabled {
		return nil
	}
	candidate := *m
	if overrideKnown {
		candidate.APIKeyOverride = strings.TrimSpace(plainOverride)
	} else if strings.TrimSpace(candidate.APIKeyOverride) != "" {
		if s.encryptor == nil {
			return ErrAccountQuotaMonitorKeyDecryptFailed
		}
		plain, err := s.encryptor.Decrypt(candidate.APIKeyOverride)
		if err != nil {
			slog.Warn("account_quota_monitor: decrypt api key override failed before fetch validation", "monitor_id", m.ID, "error", err)
			return ErrAccountQuotaMonitorKeyDecryptFailed
		}
		candidate.APIKeyOverride = strings.TrimSpace(plain)
	}
	endpoint, apiKey := resolveQuotaMonitorEndpointAndKey(&candidate, account)
	validationCtx, cancel := context.WithTimeout(ctx, quotaMonitorRunOneTimeout)
	defer cancel()
	if _, err := s.fetcher.Fetch(validationCtx, accountQuotaFetchInput{
		Provider:  candidate.Provider,
		Endpoint:  endpoint,
		APIKey:    apiKey,
		Currency:  candidate.Currency,
		Threshold: candidate.LowBalanceThreshold,
	}); err != nil {
		slog.Warn("account_quota_monitor: fetch validation failed", "monitor_id", m.ID, "account_id", m.AccountID, "endpoint", normalizeQuotaMonitorEndpointKey(endpoint), "error", sanitizeQuotaMonitorError(err.Error(), apiKey))
		return ErrAccountQuotaMonitorFetchFailed
	}
	return nil
}

func (s *AccountQuotaMonitorService) runCheckForMonitor(ctx context.Context, m *AccountQuotaMonitor) *AccountQuotaCheckResult {
	checkedAt := time.Now().UTC()
	result := &AccountQuotaCheckResult{
		MonitorID: m.ID,
		AccountID: m.AccountID,
		Currency:  normalizeQuotaCurrency(m.Currency),
		Status:    QuotaMonitorStatusError,
		CheckedAt: checkedAt,
	}
	account, err := s.accountRepo.GetByID(ctx, m.AccountID)
	if err != nil {
		result.Message = truncateMessage(sanitizeErrorMessage(err.Error()))
		return result
	}
	endpoint, apiKey := resolveQuotaMonitorEndpointAndKey(m, account)
	fetchResult, err := s.fetcher.Fetch(ctx, accountQuotaFetchInput{
		Provider:  m.Provider,
		Endpoint:  endpoint,
		APIKey:    apiKey,
		Currency:  m.Currency,
		Threshold: m.LowBalanceThreshold,
	})
	if err != nil {
		result.Message = sanitizeQuotaMonitorError(err.Error(), apiKey)
		return result
	}
	fetchResult.MonitorID = m.ID
	fetchResult.AccountID = m.AccountID
	fetchResult.CheckedAt = checkedAt
	fetchResult.Message = truncateMessage(fetchResult.Message)
	return fetchResult
}

func sanitizeQuotaMonitorError(msg, apiKey string) string {
	msg = sanitizeErrorMessage(msg)
	apiKey = strings.TrimSpace(apiKey)
	if len(apiKey) >= 8 {
		msg = strings.ReplaceAll(msg, apiKey, "***REDACTED***")
		if escaped := url.QueryEscape(apiKey); escaped != apiKey {
			msg = strings.ReplaceAll(msg, escaped, "***REDACTED***")
		}
	}
	return truncateMessage(msg)
}

func resolveQuotaMonitorEndpointAndKey(m *AccountQuotaMonitor, account *Account) (endpoint, apiKey string) {
	if m != nil {
		endpoint = strings.TrimSpace(m.Endpoint)
		apiKey = strings.TrimSpace(m.APIKeyOverride)
	}
	if account == nil {
		return endpoint, apiKey
	}
	if endpoint == "" {
		endpoint = strings.TrimSpace(account.GetCredential("base_url"))
	}
	if endpoint == "" && account.IsCustomBaseURLEnabled() {
		endpoint = strings.TrimSpace(account.GetCustomBaseURL())
	}
	if apiKey == "" {
		apiKey = firstNonEmpty(
			account.GetCredential("api_key"),
			account.GetCredential("access_token"),
			account.GetCredential("session_key"),
			account.GetCredential("token"),
		)
	}
	return endpoint, apiKey
}

func (s *AccountQuotaMonitorService) encryptOverride(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	encrypted, err := s.encryptor.Encrypt(raw)
	if err != nil {
		return "", fmt.Errorf("encrypt quota monitor api key override: %w", err)
	}
	return encrypted, nil
}

func (s *AccountQuotaMonitorService) decryptOverridesInPlace(items []*AccountQuotaMonitor) {
	for _, m := range items {
		s.decryptOverrideInPlace(m)
	}
}

func (s *AccountQuotaMonitorService) decryptOverrideInPlace(m *AccountQuotaMonitor) {
	if m == nil || strings.TrimSpace(m.APIKeyOverride) == "" {
		return
	}
	plain, err := s.encryptor.Decrypt(m.APIKeyOverride)
	if err != nil {
		slog.Warn("account_quota_monitor: decrypt api key override failed", "monitor_id", m.ID, "error", err)
		m.APIKeyOverride = ""
		m.APIKeyOverrideSet = true
		m.APIKeyOverrideDecryptFailed = true
		return
	}
	m.APIKeyOverride = plain
	m.APIKeyOverrideSet = true
}

func (s *AccountQuotaMonitorService) enrichMetrics(ctx context.Context, items []*AccountQuotaMonitor) {
	if len(items) == 0 {
		return
	}
	ids := make([]int64, 0, len(items))
	for _, m := range items {
		ids = append(ids, m.ID)
	}
	metrics, err := s.repo.ComputeMetricsFor(ctx, ids)
	if err != nil {
		slog.Warn("account_quota_monitor: compute metrics failed", "error", err)
		return
	}
	for _, m := range items {
		if v, ok := metrics[m.ID]; ok {
			m.Metrics = v
		}
	}
}

func normalizeQuotaInterval(sec int) int {
	if sec <= 0 {
		return 3600
	}
	return sec
}

func cloneFloat64Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}
