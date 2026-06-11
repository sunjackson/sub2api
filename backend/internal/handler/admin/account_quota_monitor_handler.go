package admin

import (
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const quotaMonitorMaxPageSize = 100

type AccountQuotaMonitorHandler struct {
	quotaService *service.AccountQuotaMonitorService
}

func NewAccountQuotaMonitorHandler(quotaService *service.AccountQuotaMonitorService) *AccountQuotaMonitorHandler {
	return &AccountQuotaMonitorHandler{quotaService: quotaService}
}

type accountQuotaMonitorBatchAccountFiltersRequest struct {
	Platform    string `json:"platform"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Group       string `json:"group"`
	Search      string `json:"search"`
	PrivacyMode string `json:"privacy_mode"`
}

type accountQuotaMonitorBatchCreateRequest struct {
	AccountIDs          []int64                                        `json:"account_ids"`
	Filters             *accountQuotaMonitorBatchAccountFiltersRequest `json:"filters"`
	Provider            string                                         `json:"provider" binding:"required,oneof=sub2api newapi custom"`
	Endpoint            string                                         `json:"endpoint" binding:"omitempty,max=1000"`
	APIKeyOverride      string                                         `json:"api_key_override" binding:"omitempty,max=4000"`
	Enabled             *bool                                          `json:"enabled"`
	IntervalSeconds     int                                            `json:"interval_seconds" binding:"omitempty,min=60,max=86400"`
	LowBalanceThreshold *float64                                       `json:"low_balance_threshold"`
	Currency            string                                         `json:"currency" binding:"omitempty,max=16"`
	UpdateExisting      bool                                           `json:"update_existing"`
	MaxAccounts         int                                            `json:"max_accounts" binding:"omitempty,min=1,max=1000"`
}

type accountQuotaMonitorCreateRequest struct {
	Name                string   `json:"name" binding:"required,max=120"`
	AccountID           int64    `json:"account_id" binding:"required,min=1"`
	Provider            string   `json:"provider" binding:"required,oneof=sub2api newapi custom"`
	Endpoint            string   `json:"endpoint" binding:"omitempty,max=1000"`
	APIKeyOverride      string   `json:"api_key_override" binding:"omitempty,max=4000"`
	Enabled             *bool    `json:"enabled"`
	IntervalSeconds     int      `json:"interval_seconds" binding:"omitempty,min=60,max=86400"`
	LowBalanceThreshold *float64 `json:"low_balance_threshold"`
	Currency            string   `json:"currency" binding:"omitempty,max=16"`
}

type accountQuotaMonitorUpdateRequest struct {
	Name                     *string  `json:"name" binding:"omitempty,max=120"`
	AccountID                *int64   `json:"account_id" binding:"omitempty,min=1"`
	Provider                 *string  `json:"provider" binding:"omitempty,oneof=sub2api newapi custom"`
	Endpoint                 *string  `json:"endpoint" binding:"omitempty,max=1000"`
	APIKeyOverride           *string  `json:"api_key_override" binding:"omitempty,max=4000"`
	ClearAPIKeyOverride      bool     `json:"clear_api_key_override"`
	Enabled                  *bool    `json:"enabled"`
	IntervalSeconds          *int     `json:"interval_seconds" binding:"omitempty,min=60,max=86400"`
	LowBalanceThreshold      *float64 `json:"low_balance_threshold"`
	ClearLowBalanceThreshold bool     `json:"clear_low_balance_threshold"`
	Currency                 *string  `json:"currency" binding:"omitempty,max=16"`
}

type accountQuotaMonitorResponse struct {
	ID                          int64    `json:"id"`
	Name                        string   `json:"name"`
	AccountID                   int64    `json:"account_id"`
	AccountName                 string   `json:"account_name"`
	AccountPlatform             string   `json:"account_platform"`
	AccountType                 string   `json:"account_type"`
	Provider                    string   `json:"provider"`
	Endpoint                    string   `json:"endpoint"`
	APIKeyOverrideSet           bool     `json:"api_key_override_set"`
	APIKeyOverrideMasked        string   `json:"api_key_override_masked"`
	APIKeyOverrideDecryptFailed bool     `json:"api_key_override_decrypt_failed"`
	Enabled                     bool     `json:"enabled"`
	IntervalSeconds             int      `json:"interval_seconds"`
	LowBalanceThreshold         *float64 `json:"low_balance_threshold"`
	Currency                    string   `json:"currency"`
	LastBalance                 *float64 `json:"last_balance"`
	LastQuotaTotal              *float64 `json:"last_quota_total"`
	LastQuotaUsed               *float64 `json:"last_quota_used"`
	LastCheckedAt               *string  `json:"last_checked_at"`
	LastStatus                  string   `json:"last_status"`
	LastMessage                 string   `json:"last_message"`
	CreatedBy                   int64    `json:"created_by"`
	CreatedAt                   string   `json:"created_at"`
	UpdatedAt                   string   `json:"updated_at"`
	Consumption24h              *float64 `json:"consumption_24h"`
	AvgDailyConsumption         *float64 `json:"avg_daily_consumption"`
	EstimatedDaysRemaining      *float64 `json:"estimated_days_remaining"`
	EstimatedDepletedAt         *string  `json:"estimated_depleted_at"`
}

type accountQuotaCheckResultResponse struct {
	MonitorID  int64    `json:"monitor_id"`
	AccountID  int64    `json:"account_id"`
	Balance    *float64 `json:"balance"`
	QuotaTotal *float64 `json:"quota_total"`
	QuotaUsed  *float64 `json:"quota_used"`
	Currency   string   `json:"currency"`
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	CheckedAt  string   `json:"checked_at"`
}

type accountQuotaHistoryItemResponse struct {
	ID         int64    `json:"id"`
	MonitorID  int64    `json:"monitor_id"`
	AccountID  int64    `json:"account_id"`
	Balance    *float64 `json:"balance"`
	QuotaTotal *float64 `json:"quota_total"`
	QuotaUsed  *float64 `json:"quota_used"`
	Currency   string   `json:"currency"`
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	CheckedAt  string   `json:"checked_at"`
}

type accountQuotaTrendPointResponse struct {
	Bucket   string  `json:"bucket"`
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
	Count    int64   `json:"count"`
}
type accountQuotaMonitorBatchCreateResponse struct {
	Selected        int64                                     `json:"selected"`
	Created         int64                                     `json:"created"`
	Updated         int64                                     `json:"updated"`
	SkippedExisting int64                                     `json:"skipped_existing"`
	Failed          int64                                     `json:"failed"`
	Items           []*accountQuotaMonitorResponse            `json:"items"`
	Failures        []service.AccountQuotaMonitorBatchFailure `json:"failures"`
}

func (h *AccountQuotaMonitorHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > quotaMonitorMaxPageSize {
		pageSize = quotaMonitorMaxPageSize
	}
	accountID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("account_id")), 10, 64)
	params := service.AccountQuotaMonitorListParams{
		Page:      page,
		PageSize:  pageSize,
		Provider:  strings.TrimSpace(c.Query("provider")),
		Enabled:   parseListEnabled(c.Query("enabled")),
		AccountID: accountID,
		Search:    strings.TrimSpace(c.Query("search")),
	}
	items, total, err := h.quotaService.List(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]*accountQuotaMonitorResponse, 0, len(items))
	for _, item := range items {
		out = append(out, accountQuotaMonitorToResponse(item))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *AccountQuotaMonitorHandler) Get(c *gin.Context) {
	id, ok := parseAccountQuotaMonitorID(c)
	if !ok {
		return
	}
	m, err := h.quotaService.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, accountQuotaMonitorToResponse(m))
}

func (h *AccountQuotaMonitorHandler) Create(c *gin.Context) {
	var req accountQuotaMonitorCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	interval := req.IntervalSeconds
	if interval == 0 {
		interval = 3600
	}
	m, err := h.quotaService.Create(c.Request.Context(), service.AccountQuotaMonitorCreateParams{
		Name:                req.Name,
		AccountID:           req.AccountID,
		Provider:            req.Provider,
		Endpoint:            req.Endpoint,
		APIKeyOverride:      req.APIKeyOverride,
		Enabled:             enabled,
		IntervalSeconds:     interval,
		LowBalanceThreshold: req.LowBalanceThreshold,
		Currency:            req.Currency,
		CreatedBy:           subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, accountQuotaMonitorToResponse(m))
}

func (h *AccountQuotaMonitorHandler) BatchCreate(c *gin.Context) {
	var req accountQuotaMonitorBatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	interval := req.IntervalSeconds
	if interval == 0 {
		interval = 3600
	}
	filters, err := quotaMonitorBatchFiltersFromRequest(req.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.quotaService.BatchCreate(c.Request.Context(), service.AccountQuotaMonitorBatchCreateParams{
		AccountIDs:          req.AccountIDs,
		Filters:             filters,
		Provider:            req.Provider,
		Endpoint:            req.Endpoint,
		APIKeyOverride:      req.APIKeyOverride,
		Enabled:             enabled,
		IntervalSeconds:     interval,
		LowBalanceThreshold: req.LowBalanceThreshold,
		Currency:            req.Currency,
		CreatedBy:           subject.UserID,
		UpdateExisting:      req.UpdateExisting,
		MaxAccounts:         req.MaxAccounts,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, accountQuotaBatchCreateToResponse(result))
}

func (h *AccountQuotaMonitorHandler) Update(c *gin.Context) {
	id, ok := parseAccountQuotaMonitorID(c)
	if !ok {
		return
	}
	var req accountQuotaMonitorUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	m, err := h.quotaService.Update(c.Request.Context(), id, service.AccountQuotaMonitorUpdateParams{
		Name:                     req.Name,
		AccountID:                req.AccountID,
		Provider:                 req.Provider,
		Endpoint:                 req.Endpoint,
		APIKeyOverride:           req.APIKeyOverride,
		ClearAPIKeyOverride:      req.ClearAPIKeyOverride,
		Enabled:                  req.Enabled,
		IntervalSeconds:          req.IntervalSeconds,
		LowBalanceThreshold:      req.LowBalanceThreshold,
		ClearLowBalanceThreshold: req.ClearLowBalanceThreshold,
		Currency:                 req.Currency,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, accountQuotaMonitorToResponse(m))
}

func (h *AccountQuotaMonitorHandler) Delete(c *gin.Context) {
	id, ok := parseAccountQuotaMonitorID(c)
	if !ok {
		return
	}
	if err := h.quotaService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *AccountQuotaMonitorHandler) Run(c *gin.Context) {
	id, ok := parseAccountQuotaMonitorID(c)
	if !ok {
		return
	}
	result, err := h.quotaService.RunCheck(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, accountQuotaCheckResultToResponse(result))
}

func (h *AccountQuotaMonitorHandler) History(c *gin.Context) {
	id, ok := parseAccountQuotaMonitorID(c)
	if !ok {
		return
	}
	entries, err := h.quotaService.ListHistory(c.Request.Context(), id, parseQuotaHistoryLimit(c.Query("limit")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]accountQuotaHistoryItemResponse, 0, len(entries))
	for _, entry := range entries {
		out = append(out, accountQuotaHistoryToResponse(entry))
	}
	response.Success(c, gin.H{"items": out})
}

func (h *AccountQuotaMonitorHandler) Summary(c *gin.Context) {
	summary, err := h.quotaService.Summary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

func (h *AccountQuotaMonitorHandler) Trend(c *gin.Context) {
	days, _ := strconv.Atoi(strings.TrimSpace(c.Query("days")))
	bucket := strings.TrimSpace(c.Query("bucket"))
	points, err := h.quotaService.Trend(c.Request.Context(), days, bucket)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]accountQuotaTrendPointResponse, 0, len(points))
	for _, p := range points {
		out = append(out, accountQuotaTrendToResponse(p))
	}
	response.Success(c, gin.H{"items": out})
}

func quotaMonitorBatchFiltersFromRequest(req *accountQuotaMonitorBatchAccountFiltersRequest) (service.AccountQuotaMonitorAccountFilters, error) {
	if req == nil {
		return service.AccountQuotaMonitorAccountFilters{}, nil
	}
	filters := service.AccountQuotaMonitorAccountFilters{
		Platform:    strings.TrimSpace(req.Platform),
		AccountType: strings.TrimSpace(req.Type),
		Status:      strings.TrimSpace(req.Status),
		Search:      strings.TrimSpace(req.Search),
		PrivacyMode: strings.TrimSpace(req.PrivacyMode),
	}
	if group := strings.TrimSpace(req.Group); group != "" {
		if group == accountListGroupUngroupedQueryValue {
			filters.GroupID = service.AccountListGroupUngrouped
		} else {
			parsed, err := strconv.ParseInt(group, 10, 64)
			if err != nil || parsed < 0 {
				return filters, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter")
			}
			filters.GroupID = parsed
		}
	}
	return filters, nil
}

func accountQuotaBatchCreateToResponse(result *service.AccountQuotaMonitorBatchCreateResult) accountQuotaMonitorBatchCreateResponse {
	resp := accountQuotaMonitorBatchCreateResponse{}
	if result == nil {
		return resp
	}
	resp.Selected = result.Selected
	resp.Created = result.Created
	resp.Updated = result.Updated
	resp.SkippedExisting = result.SkippedExisting
	resp.Failed = result.Failed
	resp.Failures = result.Failures
	resp.Items = make([]*accountQuotaMonitorResponse, 0, len(result.Items))
	for _, item := range result.Items {
		resp.Items = append(resp.Items, accountQuotaMonitorToResponse(item))
	}
	return resp
}

func parseAccountQuotaMonitorID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_QUOTA_MONITOR_ID", "invalid quota monitor id"))
		return 0, false
	}
	return id, true
}

func parseQuotaHistoryLimit(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return service.QuotaMonitorHistoryDefaultLimit
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return service.QuotaMonitorHistoryDefaultLimit
	}
	if v > service.QuotaMonitorHistoryMaxLimit {
		return service.QuotaMonitorHistoryMaxLimit
	}
	return v
}

func accountQuotaMonitorToResponse(m *service.AccountQuotaMonitor) *accountQuotaMonitorResponse {
	if m == nil {
		return nil
	}
	resp := &accountQuotaMonitorResponse{
		ID:                          m.ID,
		Name:                        m.Name,
		AccountID:                   m.AccountID,
		AccountName:                 m.AccountName,
		AccountPlatform:             m.AccountPlatform,
		AccountType:                 m.AccountType,
		Provider:                    m.Provider,
		Endpoint:                    m.Endpoint,
		APIKeyOverrideSet:           m.APIKeyOverrideSet,
		APIKeyOverrideMasked:        quotaMonitorMaskedOverride(m),
		APIKeyOverrideDecryptFailed: m.APIKeyOverrideDecryptFailed,
		Enabled:                     m.Enabled,
		IntervalSeconds:             m.IntervalSeconds,
		LowBalanceThreshold:         m.LowBalanceThreshold,
		Currency:                    m.Currency,
		LastBalance:                 m.LastBalance,
		LastQuotaTotal:              m.LastQuotaTotal,
		LastQuotaUsed:               m.LastQuotaUsed,
		LastStatus:                  m.LastStatus,
		LastMessage:                 m.LastMessage,
		CreatedBy:                   m.CreatedBy,
		CreatedAt:                   formatQuotaTime(m.CreatedAt),
		UpdatedAt:                   formatQuotaTime(m.UpdatedAt),
		Consumption24h:              m.Metrics.Consumption24h,
		AvgDailyConsumption:         m.Metrics.AvgDailyConsumption,
		EstimatedDaysRemaining:      m.Metrics.EstimatedDaysRemaining,
	}
	if m.LastCheckedAt != nil {
		s := formatQuotaTime(*m.LastCheckedAt)
		resp.LastCheckedAt = &s
	}
	if m.Metrics.EstimatedDepletedAt != nil {
		s := formatQuotaTime(*m.Metrics.EstimatedDepletedAt)
		resp.EstimatedDepletedAt = &s
	}
	return resp
}

func quotaMonitorMaskedOverride(m *service.AccountQuotaMonitor) string {
	if m == nil || !m.APIKeyOverrideSet {
		return ""
	}
	return maskAPIKey(m.APIKeyOverride)
}

func accountQuotaCheckResultToResponse(r *service.AccountQuotaCheckResult) accountQuotaCheckResultResponse {
	return accountQuotaCheckResultResponse{
		MonitorID:  r.MonitorID,
		AccountID:  r.AccountID,
		Balance:    r.Balance,
		QuotaTotal: r.QuotaTotal,
		QuotaUsed:  r.QuotaUsed,
		Currency:   r.Currency,
		Status:     r.Status,
		Message:    r.Message,
		CheckedAt:  formatQuotaTime(r.CheckedAt),
	}
}

func accountQuotaHistoryToResponse(e *service.AccountQuotaMonitorHistoryEntry) accountQuotaHistoryItemResponse {
	return accountQuotaHistoryItemResponse{
		ID:         e.ID,
		MonitorID:  e.MonitorID,
		AccountID:  e.AccountID,
		Balance:    e.Balance,
		QuotaTotal: e.QuotaTotal,
		QuotaUsed:  e.QuotaUsed,
		Currency:   e.Currency,
		Status:     e.Status,
		Message:    e.Message,
		CheckedAt:  formatQuotaTime(e.CheckedAt),
	}
}

func accountQuotaTrendToResponse(p *service.AccountQuotaTrendPoint) accountQuotaTrendPointResponse {
	return accountQuotaTrendPointResponse{
		Bucket:   formatQuotaTime(p.Bucket),
		Currency: p.Currency,
		Balance:  p.Balance,
		Count:    p.Count,
	}
}

func formatQuotaTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
