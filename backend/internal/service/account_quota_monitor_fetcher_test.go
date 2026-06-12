package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseQuotaPayload_FlexibleShapes(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantBalance  float64
		wantTotal    *float64
		wantUsed     *float64
		wantCurrency string
	}{
		{
			name:         "flat balance",
			body:         `{"balance":12.34,"currency":"USD"}`,
			wantBalance:  12.34,
			wantCurrency: "USD",
		},
		{
			name:         "nested remaining quota string",
			body:         `{"data":{"remaining_quota":"88.5","unit":"CNY"}}`,
			wantBalance:  88.5,
			wantCurrency: "CNY",
		},
		{
			name:         "derive balance from total and used",
			body:         `{"data":{"total_quota":100,"used_quota":40,"currency":"USD"}}`,
			wantBalance:  60,
			wantTotal:    floatPtr(100),
			wantUsed:     floatPtr(40),
			wantCurrency: "USD",
		},
		{
			name:        "openai credit grants shape",
			body:        `{"total_granted":20,"total_used":3.5,"total_available":16.5}`,
			wantBalance: 16.5,
			wantTotal:   floatPtr(20),
			wantUsed:    floatPtr(3.5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseQuotaPayload([]byte(tt.body))
			require.NoError(t, err)
			require.NotNil(t, got.Balance)
			require.InDelta(t, tt.wantBalance, *got.Balance, 0.000001)
			if tt.wantTotal != nil {
				require.NotNil(t, got.QuotaTotal)
				require.InDelta(t, *tt.wantTotal, *got.QuotaTotal, 0.000001)
			}
			if tt.wantUsed != nil {
				require.NotNil(t, got.QuotaUsed)
				require.InDelta(t, *tt.wantUsed, *got.QuotaUsed, 0.000001)
			}
			require.Equal(t, tt.wantCurrency, got.Currency)
		})
	}
}

func TestParseQuotaPayload_ScalesNewAPIQuotaUnits(t *testing.T) {
	got, err := parseQuotaPayloadWithOptions([]byte(`{
		"code": true,
		"message": "ok",
		"data": {
			"object": "token_usage",
			"total_granted": 1000000,
			"total_used": 250000,
			"total_available": 750000,
			"unlimited_quota": false
		}
	}`), quotaPayloadParseOptions{Provider: QuotaMonitorProviderNewAPI, Endpoint: "https://relay.example.com/api/usage/token/"})
	require.NoError(t, err)
	require.NotNil(t, got.Balance)
	require.NotNil(t, got.QuotaTotal)
	require.NotNil(t, got.QuotaUsed)
	require.InDelta(t, 1.5, *got.Balance, 0.000001)
	require.InDelta(t, 2.0, *got.QuotaTotal, 0.000001)
	require.InDelta(t, 0.5, *got.QuotaUsed, 0.000001)
	require.Equal(t, "USD", got.Currency)
}

func TestParseQuotaPayload_DerivesBalanceFromReversedTotalAndUsed(t *testing.T) {
	got, err := parseQuotaPayloadWithOptions([]byte(`{"data":{"total":0,"used":9.47115}}`), quotaPayloadParseOptions{Provider: QuotaMonitorProviderCustom, Endpoint: "https://relay.example.com/api/user/dashboard"})
	require.NoError(t, err)
	require.NotNil(t, got.Balance)
	require.NotNil(t, got.QuotaTotal)
	require.NotNil(t, got.QuotaUsed)
	require.InDelta(t, 9.47115, *got.Balance, 0.000001)
	require.InDelta(t, 9.47115, *got.QuotaTotal, 0.000001)
	require.InDelta(t, 0, *got.QuotaUsed, 0.000001)
}

func TestParseQuotaPayload_NormalizesNegativeBalanceFromReversedTotalAndUsed(t *testing.T) {
	got, err := parseQuotaPayloadWithOptions([]byte(`{"data":{"balance":-9.47115,"total":0,"used":9.47115}}`), quotaPayloadParseOptions{Provider: QuotaMonitorProviderCustom, Endpoint: "https://relay.example.com/api/user/dashboard"})
	require.NoError(t, err)
	require.NotNil(t, got.Balance)
	require.NotNil(t, got.QuotaTotal)
	require.NotNil(t, got.QuotaUsed)
	require.InDelta(t, 9.47115, *got.Balance, 0.000001)
	require.InDelta(t, 9.47115, *got.QuotaTotal, 0.000001)
	require.InDelta(t, 0, *got.QuotaUsed, 0.000001)
}

func TestParseQuotaPayload_NormalizesNegativeNewAPITokenUsage(t *testing.T) {
	got, err := parseQuotaPayloadWithOptions([]byte(`{
		"data": {
			"object": "token_usage",
			"total_available": -905861,
			"total_granted": 0,
			"total_used": 905861,
			"unlimited_quota": false
		}
	}`), quotaPayloadParseOptions{Provider: QuotaMonitorProviderNewAPI, Endpoint: "https://relay.example.com/api/usage/token/"})
	require.NoError(t, err)
	require.NotNil(t, got.Balance)
	require.NotNil(t, got.QuotaTotal)
	require.NotNil(t, got.QuotaUsed)
	require.InDelta(t, 1.811722, *got.Balance, 0.000001)
	require.InDelta(t, 1.811722, *got.QuotaTotal, 0.000001)
	require.InDelta(t, 0, *got.QuotaUsed, 0.000001)
}

func TestParseQuotaPayload_RejectsGenericNegativeOverspend(t *testing.T) {
	_, err := parseQuotaPayloadWithOptions([]byte(`{"data":{"balance":-5,"total":10,"used":15}}`), quotaPayloadParseOptions{Provider: QuotaMonitorProviderCustom, Endpoint: "https://relay.example.com/custom/quota"})
	require.Error(t, err)
}

func TestParseQuotaPayload_NewAPIUserQuotaUsesQuotaAsRemaining(t *testing.T) {
	got, err := parseQuotaPayloadWithOptions([]byte(`{"data":{"quota":1000000,"used_quota":250000}}`), quotaPayloadParseOptions{Provider: QuotaMonitorProviderNewAPI, Endpoint: "https://relay.example.com/api/user/self"})
	require.NoError(t, err)
	require.NotNil(t, got.Balance)
	require.NotNil(t, got.QuotaUsed)
	require.InDelta(t, 2.0, *got.Balance, 0.000001)
	require.InDelta(t, 0.5, *got.QuotaUsed, 0.000001)
	require.Equal(t, "USD", got.Currency)
}

func TestParseQuotaPayload_DerivesReversedDashboardUsage(t *testing.T) {
	got, err := parseQuotaPayloadWithOptions([]byte(`{"data":{"total":0,"used":250000}}`), quotaPayloadParseOptions{Provider: QuotaMonitorProviderNewAPI, Endpoint: "https://relay.example.com/api/user/dashboard"})
	require.NoError(t, err)
	require.NotNil(t, got.Balance)
	require.NotNil(t, got.QuotaTotal)
	require.NotNil(t, got.QuotaUsed)
	require.InDelta(t, 0.5, *got.Balance, 0.000001)
	require.InDelta(t, 0.5, *got.QuotaTotal, 0.000001)
	require.InDelta(t, 0, *got.QuotaUsed, 0.000001)
}

func TestParseQuotaPayload_DoesNotScaleOpenAICreditGrants(t *testing.T) {
	body := []byte(`{"total_granted":20,"total_used":3.5,"total_available":16.5}`)

	defaultParsed, err := parseQuotaPayload(body)
	require.NoError(t, err)
	require.NotNil(t, defaultParsed.Balance)
	require.InDelta(t, 16.5, *defaultParsed.Balance, 0.000001)

	creditGrantsParsed, err := parseQuotaPayloadWithOptions(body, quotaPayloadParseOptions{Provider: QuotaMonitorProviderNewAPI, Endpoint: "https://api.openai.com/v1/dashboard/billing/credit_grants"})
	require.NoError(t, err)
	require.NotNil(t, creditGrantsParsed.Balance)
	require.NotNil(t, creditGrantsParsed.QuotaTotal)
	require.NotNil(t, creditGrantsParsed.QuotaUsed)
	require.InDelta(t, 16.5, *creditGrantsParsed.Balance, 0.000001)
	require.InDelta(t, 20, *creditGrantsParsed.QuotaTotal, 0.000001)
	require.InDelta(t, 3.5, *creditGrantsParsed.QuotaUsed, 0.000001)
}

func TestParseQuotaPayload_RejectsUnrecognizedPayload(t *testing.T) {
	_, err := parseQuotaPayload([]byte(`{"data":{"message":"ok"}}`))
	require.Error(t, err)
}

func floatPtr(v float64) *float64 { return &v }

func TestSanitizeQuotaMonitorErrorRedactsExactRelayKey(t *testing.T) {
	apiKey := "relay-token-without-standard-prefix-abcdef123456"
	msg := sanitizeQuotaMonitorError("upstream echoed key "+apiKey+" in error body", apiKey)
	require.NotContains(t, msg, apiKey)
	require.Contains(t, msg, "***REDACTED***")
}

func TestQuotaCandidateURLsIncludesNewAPITokenUsageFirst(t *testing.T) {
	got := quotaCandidateURLs("https://relay.example.com/v1", QuotaMonitorProviderNewAPI)
	require.NotEmpty(t, got)
	require.Equal(t, "https://relay.example.com/api/usage/token/", got[0])
}
