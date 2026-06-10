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
