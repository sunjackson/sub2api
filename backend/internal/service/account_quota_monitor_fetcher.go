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
	Balance    *float64
	QuotaTotal *float64
	QuotaUsed  *float64
	Currency   string
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
		payload, err := f.fetchOne(ctx, candidate, in.APIKey)
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

func (f *accountQuotaFetcher) fetchOne(ctx context.Context, endpoint, apiKey string) (*parsedQuotaPayload, error) {
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
	parsed, err := parseQuotaPayload(body)
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
	var decoded any
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("parse quota response JSON: %w", err)
	}
	parsed := scanQuotaValue(decoded)
	if parsed == nil || (parsed.Balance == nil && parsed.QuotaTotal == nil && parsed.QuotaUsed == nil) {
		return nil, fmt.Errorf("quota response did not contain a recognizable balance or quota field")
	}
	if parsed.Balance == nil && parsed.QuotaTotal != nil && parsed.QuotaUsed != nil {
		remaining := *parsed.QuotaTotal - *parsed.QuotaUsed
		parsed.Balance = &remaining
	}
	return parsed, nil
}

func scanQuotaValue(v any) *parsedQuotaPayload {
	switch val := v.(type) {
	case map[string]any:
		if p := parseQuotaMap(val); p != nil && (p.Balance != nil || p.QuotaTotal != nil || p.QuotaUsed != nil) {
			return p
		}
		for _, child := range val {
			if p := scanQuotaValue(child); p != nil {
				return p
			}
		}
	case []any:
		for _, child := range val {
			if p := scanQuotaValue(child); p != nil {
				return p
			}
		}
	}
	return nil
}

func parseQuotaMap(m map[string]any) *parsedQuotaPayload {
	p := &parsedQuotaPayload{}
	p.Balance = firstNumberFromMap(m,
		"balance", "remaining_balance", "remaining", "available_balance",
		"total_available", "available", "credit", "credits", "quota", "remaining_quota",
		"quota_remaining", "remain_quota", "left_quota",
	)
	p.QuotaTotal = firstNumberFromMap(m,
		"quota_total", "total_quota", "total", "total_granted", "granted", "hard_limit_usd", "limit", "quota_limit",
	)
	p.QuotaUsed = firstNumberFromMap(m,
		"quota_used", "used_quota", "used", "total_used", "usage", "used_amount", "consumed", "spent",
	)
	p.Currency = firstStringFromMap(m, "currency", "currency_code", "unit")
	return p
}

func firstNumberFromMap(m map[string]any, keys ...string) *float64 {
	lower := make(map[string]any, len(m))
	for k, v := range m {
		lower[strings.ToLower(k)] = v
	}
	for _, k := range keys {
		if n, ok := numberFromAny(lower[strings.ToLower(k)]); ok {
			return &n
		}
	}
	return nil
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
