package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type injectionSettingRepoStub struct {
	values map[string]string
}

func (s *injectionSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *injectionSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *injectionSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *injectionSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *injectionSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *injectionSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *injectionSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_GetPublicSettingsForInjection_ExcludesHeavyFields(t *testing.T) {
	repo := &injectionSettingRepoStub{values: map[string]string{
		SettingKeySiteLogo:                "data:image/png;base64," + strings.Repeat("A", 64*1024),
		SettingKeyHomeContent:             strings.Repeat("# hello\n", 4096),
		SettingKeyLoginAgreementDocuments: `[{"id":"terms","title":"Terms","content_md":"` + strings.Repeat("legal text ", 1024) + `"}]`,
		SettingKeySiteName:                "GMT",
		SettingKeyChannelMonitorEnabled:   "true",
	}}
	svc := NewSettingService(repo, &config.Config{})

	payload, err := svc.GetPublicSettingsForInjection(context.Background())
	if err != nil {
		t.Fatalf("GetPublicSettingsForInjection returned error: %v", err)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal injection payload: %v", err)
	}
	jsonText := string(encoded)
	for _, forbidden := range []string{"site_logo", "home_content", "login_agreement_documents"} {
		if strings.Contains(jsonText, forbidden) {
			t.Fatalf("injection payload contains heavy field %q: %s", forbidden, jsonText)
		}
	}
	if len(encoded) > 8192 {
		t.Fatalf("injection payload too large: got %d bytes, want <= 8192", len(encoded))
	}
}
