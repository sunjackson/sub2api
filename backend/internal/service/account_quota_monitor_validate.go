package service

import (
	"context"
	"net/url"
	"strings"
)

func validateQuotaMonitorProvider(provider string) error {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case QuotaMonitorProviderSub2API, QuotaMonitorProviderNewAPI, QuotaMonitorProviderCustom:
		return nil
	default:
		return ErrAccountQuotaMonitorInvalidProvider
	}
}

func normalizeQuotaMonitorProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func validateQuotaMonitorInterval(sec int) error {
	if sec < quotaMonitorMinIntervalSeconds || sec > quotaMonitorMaxIntervalSeconds {
		return ErrAccountQuotaMonitorInvalidInterval
	}
	return nil
}

func normalizeQuotaCurrency(currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return quotaMonitorDefaultCurrency
	}
	if len(currency) > 16 {
		return currency[:16]
	}
	return currency
}

func validateQuotaEndpoint(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ErrAccountQuotaMonitorInvalidEndpoint
	}
	if u.Scheme != "https" {
		return ErrAccountQuotaMonitorEndpointScheme
	}
	if u.Fragment != "" {
		return ErrAccountQuotaMonitorInvalidEndpoint
	}
	ctx, cancel := context.WithTimeout(context.Background(), monitorEndpointResolveTimeout)
	defer cancel()
	blocked, err := isPrivateOrLoopbackHost(ctx, u.Hostname())
	if err != nil {
		return ErrAccountQuotaMonitorEndpointUnreachable
	}
	if blocked {
		return ErrAccountQuotaMonitorEndpointPrivate
	}
	return nil
}

func normalizeQuotaEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.TrimRight(raw, "/")
	}
	u.Fragment = ""
	u.RawQuery = strings.TrimSpace(u.RawQuery)
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "/" {
		u.Path = ""
	}
	return strings.TrimRight(u.String(), "/")
}

func validateQuotaThreshold(v *float64) error {
	if v != nil && *v < 0 {
		return ErrAccountQuotaMonitorInvalidThreshold
	}
	return nil
}
