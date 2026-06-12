package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const newAPIQuotaPerUnit = 500000.0

var quotaMonitorHTTPClient = newSSRFSafeHTTPClient(quotaMonitorHTTPTimeout)

type accountQuotaFetcher struct {
	client *http.Client
}

type accountQuotaFetchInput struct {
	Provider  string
	Endpoint  string
	APIKey    string
	Currency  string
	Threshold *float64
}

type parsedQuotaPayload struct {
	Balance          *float64
	QuotaTotal       *float64
	QuotaUsed        *float64
	Currency         string
	balanceSourceKey string
	totalSourceKey   string
	usedSourceKey    string
}

type quotaPayloadParseOptions struct {
	Provider string
	Endpoint string
}

func newAccountQuotaFetcher() *accountQuotaFetcher {
	return &accountQuotaFetcher{client: quotaMonitorHTTPClient}
}

func (f *accountQuotaFetcher) Fetch(ctx context.Context, in accountQuotaFetchInput) (*AccountQuotaCheckResult, error) {
	endpoint := normalizeQuotaEndpoint(in.Endpoint)
	if endpoint == "" {
		return nil, ErrAccountQuotaMonitorMissingEndpoint
	}
	if strings.TrimSpace(in.APIKey) == "" {
		return nil, ErrAccountQuotaMonitorMissingAPIKey
	}
	if err := validateQuotaEndpoint(endpoint); err != nil {
		return nil, err
	}

	candidates := quotaCandidateURLs(endpoint, in.Provider)
	var lastErr error
	for _, candidate := range candidates {
		payload, err := f.fetchOne(ctx, candidate, in.APIKey, in.Provider)
		if err != nil {
			lastErr = err
			continue
		}
		currency := normalizeQuotaCurrency(firstNonEmptyQuotaString(payload.Currency, in.Currency))
		status := QuotaMonitorStatusOK
		if payload.Balance != nil && in.Threshold != nil && *payload.Balance <= *in.Threshold {
			status = QuotaMonitorStatusLowBalance
		}
		return &AccountQuotaCheckResult{
			Balance:    payload.Balance,
			QuotaTotal: payload.QuotaTotal,
			QuotaUsed:  payload.QuotaUsed,
			Currency:   currency,
			Status:     status,
			Message:    "",
			CheckedAt:  time.Now().UTC(),
		}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate quota endpoint parsed successfully")
	}
	return nil, lastErr
}

func (f *accountQuotaFetcher) fetchOne(ctx context.Context, endpoint, apiKey, provider string) (*parsedQuotaPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build quota request: %w", err)
	}
	applyQuotaAuthHeaders(req, apiKey)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("quota request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, quotaMonitorResponseMaxBytes))
	if err != nil {
		return nil, fmt.Errorf("read quota response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("quota endpoint HTTP %d: %s", resp.StatusCode, truncateMessage(sanitizeErrorMessage(string(body))))
	}
	parsed, err := parseQuotaPayloadWithOptions(body, quotaPayloadParseOptions{Provider: provider, Endpoint: endpoint})
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func applyQuotaAuthHeaders(req *http.Request, apiKey string) {
	apiKey = strings.TrimSpace(apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-API-Key", apiKey)
}

func quotaCandidateURLs(endpoint, provider string) []string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return []string{endpoint}
	}
	provider = normalizeQuotaMonitorProvider(provider)
	out := make([]string, 0, 12)
	add := func(s string) {
		for _, existing := range out {
			if existing == s {
				return
			}
		}
		out = append(out, s)
	}

	if u.Path != "" && u.Path != "/" && !isVersionOnlyPath(u.Path) {
		add(u.String())
	}
	base := quotaBaseURL(u)
	for _, path := range quotaCandidatePaths(provider) {
		add(joinQuotaURL(base, path))
	}
	if len(out) == 0 {
		add(endpoint)
	}
	return out
}

func quotaBaseURL(u *url.URL) string {
	clone := *u
	clone.RawQuery = ""
	clone.Fragment = ""
	path := strings.TrimRight(clone.Path, "/")
	switch path {
	case "/v1", "/api/v1", "/api":
		clone.Path = ""
	case "":
		clone.Path = ""
	default:
		clone.Path = path
	}
	return strings.TrimRight(clone.String(), "/")
}

func isVersionOnlyPath(path string) bool {
	path = strings.TrimRight(path, "/")
	return path == "/v1" || path == "/api" || path == "/api/v1"
}

func quotaCandidatePaths(provider string) []string {
	common := []string{
		"/api/usage/token/",
		"/api/user/self",
		"/api/user/dashboard",
		"/api/token/self",
		"/api/user/token",
		"/api/v1/user/profile",
		"/api/v1/user",
		"/api/v1/user/self",
		"/user/self",
		"/dashboard/billing/credit_grants",
		"/v1/dashboard/billing/credit_grants",
	}
	switch provider {
	case QuotaMonitorProviderSub2API:
		return append([]string{"/api/usage/token/", "/api/v1/user/profile", "/api/v1/user"}, common...)
	case QuotaMonitorProviderNewAPI:
		return append([]string{"/api/usage/token/", "/api/user/self", "/api/user/dashboard"}, common...)
	default:
		return common
	}
}

func joinQuotaURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func parseQuotaPayload(body []byte) (*parsedQuotaPayload, error) {
	return parseQuotaPayloadWithOptions(body, quotaPayloadParseOptions{})
}

func parseQuotaPayloadWithOptions(body []byte, opts quotaPayloadParseOptions) (*parsedQuotaPayload, error) {
	var decoded any
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("parse quota response JSON: %w", err)
	}
	parsed := scanQuotaValue(decoded, opts)
	if parsed == nil || (parsed.Balance == nil && parsed.QuotaTotal == nil && parsed.QuotaUsed == nil) {
		return nil, fmt.Errorf("quota response did not contain a recognizable balance or quota field")
	}
	if parsed.Balance == nil && parsed.QuotaTotal != nil && parsed.QuotaUsed != nil {
		remaining := *parsed.QuotaTotal - *parsed.QuotaUsed
		parsed.Balance = &remaining
	}
	return parsed, nil
}

func scanQuotaValue(v any, opts quotaPayloadParseOptions) *parsedQuotaPayload {
	switch val := v.(type) {
	case map[string]any:
		if p := parseQuotaMap(val, opts); p != nil && (p.Balance != nil || p.QuotaTotal != nil || p.QuotaUsed != nil) {
			return p
		}
		for _, child := range val {
			if p := scanQuotaValue(child, opts); p != nil {
				return p
			}
		}
	case []any:
		for _, child := range val {
			if p := scanQuotaValue(child, opts); p != nil {
				return p
			}
		}
	}
	return nil
}

func parseQuotaMap(m map[string]any, opts quotaPayloadParseOptions) *parsedQuotaPayload {
	p := &parsedQuotaPayload{}
	p.Balance, p.balanceSourceKey = firstNumberFromMap(m,
		"balance", "remaining_balance", "remaining", "available_balance",
		"total_available", "available", "credit", "credits", "quota", "remaining_quota",
		"quota_remaining", "remain_quota", "left_quota",
	)
	p.QuotaTotal, p.totalSourceKey = firstNumberFromMap(m,
		"quota_total", "total_quota", "total", "total_granted", "granted", "hard_limit_usd", "limit", "quota_limit",
	)
	p.QuotaUsed, p.usedSourceKey = firstNumberFromMap(m,
		"quota_used", "used_quota", "used", "total_used", "usage", "used_amount", "consumed", "spent",
	)
	p.Currency = firstStringFromMap(m, "currency", "currency_code", "unit")
	if shouldScaleNewAPIQuotaMap(m, opts, p) {
		scaleQuotaPayload(p, newAPIQuotaPerUnit)
		if strings.TrimSpace(p.Currency) == "" {
			p.Currency = "USD"
		}
	}
	return p
}

func shouldScaleNewAPIQuotaMap(m map[string]any, opts quotaPayloadParseOptions, p *parsedQuotaPayload) bool {
	if p == nil {
		return false
	}
	if hasNewAPIQuotaShapeIndicator(m) {
		return true
	}
	if !hasNewAPIQuotaSourceKey(p) {
		return false
	}
	if isOpenAICreditGrantsEndpoint(opts.Endpoint) {
		return false
	}
	if isNewAPIQuotaEndpoint(opts.Endpoint) {
		return true
	}
	switch normalizeQuotaMonitorProvider(opts.Provider) {
	case QuotaMonitorProviderNewAPI, QuotaMonitorProviderSub2API:
		return true
	default:
		return false
	}
}

func hasNewAPIQuotaShapeIndicator(m map[string]any) bool {
	lower := make(map[string]any, len(m))
	for k, v := range m {
		lower[strings.ToLower(k)] = v
	}
	if strings.EqualFold(strings.TrimSpace(stringFromUnknown(lower["object"])), "token_usage") {
		return true
	}
	for _, key := range []string{"unlimited_quota", "model_limits_enabled", "model_limits"} {
		if _, ok := lower[key]; ok {
			return true
		}
	}
	return false
}

func hasNewAPIQuotaSourceKey(p *parsedQuotaPayload) bool {
	for _, key := range []string{p.balanceSourceKey, p.totalSourceKey, p.usedSourceKey} {
		switch key {
		case "total_available", "total_granted", "total_used",
			"quota", "remaining_quota", "quota_remaining", "remain_quota", "left_quota",
			"quota_total", "total_quota", "used_quota":
			return true
		}
	}
	return false
}

func isNewAPIQuotaEndpoint(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	path := strings.TrimRight(strings.ToLower(u.Path), "/")
	switch path {
	case "/api/usage/token", "/api/user/self", "/api/user/dashboard", "/api/token/self", "/api/user/token", "/api/v1/user/profile", "/api/v1/user", "/api/v1/user/self", "/user/self":
		return true
	default:
		return false
	}
}

func isOpenAICreditGrantsEndpoint(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(u.Path), "/dashboard/billing/credit_grants")
}

func scaleQuotaPayload(p *parsedQuotaPayload, factor float64) {
	if p == nil || factor == 0 {
		return
	}
	if p.Balance != nil {
		*p.Balance = *p.Balance / factor
	}
	if p.QuotaTotal != nil {
		*p.QuotaTotal = *p.QuotaTotal / factor
	}
	if p.QuotaUsed != nil {
		*p.QuotaUsed = *p.QuotaUsed / factor
	}
}

func firstNumberFromMap(m map[string]any, keys ...string) (*float64, string) {
	lower := make(map[string]any, len(m))
	for k, v := range m {
		lower[strings.ToLower(k)] = v
	}
	for _, k := range keys {
		key := strings.ToLower(k)
		if n, ok := numberFromAny(lower[key]); ok {
			return &n, key
		}
	}
	return nil, ""
}

func firstStringFromMap(m map[string]any, keys ...string) string {
	lower := make(map[string]any, len(m))
	for k, v := range m {
		lower[strings.ToLower(k)] = v
	}
	for _, k := range keys {
		if s := strings.TrimSpace(stringFromUnknown(lower[strings.ToLower(k)])); s != "" {
			return s
		}
	}
	return ""
}

func numberFromAny(v any) (float64, bool) {
	switch val := v.(type) {
	case nil:
		return 0, false
	case json.Number:
		n, err := val.Float64()
		return n, err == nil
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case string:
		s := strings.TrimSpace(strings.ReplaceAll(val, ",", ""))
		if s == "" {
			return 0, false
		}
		n, err := strconv.ParseFloat(s, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func stringFromUnknown(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	default:
		return ""
	}
}

func firstNonEmptyQuotaString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
