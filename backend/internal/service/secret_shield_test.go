package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecretShield_ReplacesHighConfidenceSecretsAndRestores(t *testing.T) {
	vault := NewSecretShieldVault()
	body := []byte(`{"model":"gpt-5.5","input":"OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz123456 password=hunter2"}`)

	protected, changed := vault.ProtectBytes(body)
	require.True(t, changed)
	require.NotContains(t, string(protected), "sk-proj-abcdefghijklmnopqrstuvwxyz123456")
	require.NotContains(t, string(protected), "hunter2")
	require.Contains(t, string(protected), secretShieldPlaceholderPrefix)
	require.Equal(t, 2, vault.PlaceholderCount())

	restored := vault.RestoreBytes(protected)
	require.Equal(t, string(body), string(restored))
}

func TestSecretShield_DoesNotReplaceUUIDOrExamples(t *testing.T) {
	vault := NewSecretShieldVault()
	body := []byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","input":"Use YOUR_API_KEY in .env.example and dummy password=example"}`)

	protected, changed := vault.ProtectBytes(body)
	require.False(t, changed)
	require.Equal(t, string(body), string(protected))
	require.True(t, vault.Empty())
}

func TestSecretShield_ReplacesJSONPasswordEvenWhenAllLetters(t *testing.T) {
	vault := NewSecretShieldVault()
	body := []byte(`{"password":"correcthorsebatterystaple"}`)

	protected, changed := vault.ProtectBytes(body)
	require.True(t, changed)
	require.NotContains(t, string(protected), "correcthorsebatterystaple")
	require.Equal(t, string(body), string(vault.RestoreBytes(protected)))
}

func TestSecretShield_DoesNotReplacePlainCodeIdentifiers(t *testing.T) {
	vault := NewSecretShieldVault()
	body := []byte(`{"input":"const secret = getSecret(); const password = readPassword();"}`)

	protected, changed := vault.ProtectBytes(body)
	require.False(t, changed)
	require.Equal(t, string(body), string(protected))
}

func TestSecretShield_DoesNotReplaceEnvironmentReferences(t *testing.T) {
	vault := NewSecretShieldVault()
	body := []byte(`{"api_key":"process.env.OPENAI_API_KEY","input":"OPENAI_API_KEY=process.env.OPENAI_API_KEY SECRET=config.runtime.secret PASSWORD=$PASSWORD"}`)

	protected, changed := vault.ProtectBytes(body)
	require.False(t, changed)
	require.Equal(t, string(body), string(protected))
}

func TestSecretShield_PreservesJSONValidity(t *testing.T) {
	vault := NewSecretShieldVault()
	body := []byte(`{"messages":[{"role":"user","content":"Authorization: Bearer abcdefghijklmnopqrstuvwxyz1234567890"}]}`)

	protected, changed := vault.ProtectBytes(body)
	require.True(t, changed)
	require.True(t, json.Valid(protected))
	require.NotContains(t, string(protected), "abcdefghijklmnopqrstuvwxyz1234567890")
}

func TestSecretShield_StreamRestorerHandlesSplitPlaceholder(t *testing.T) {
	vault := NewSecretShieldVault()
	protected := vault.ProtectString("token sk-proj-abcdefghijklmnopqrstuvwxyz123456 echoed")
	require.Contains(t, protected, secretShieldPlaceholderPrefix)
	placeholderStart := strings.Index(protected, secretShieldPlaceholderPrefix)
	require.GreaterOrEqual(t, placeholderStart, 0)
	split := placeholderStart + len(secretShieldPlaceholderPrefix) + 4

	restorer := vault.NewStreamRestorer()
	part1 := restorer.RestoreChunk([]byte(protected[:split]), false)
	part2 := restorer.RestoreChunk([]byte(protected[split:]), false)
	final := restorer.Finalize()

	got := string(part1) + string(part2) + string(final)
	require.Equal(t, "token sk-proj-abcdefghijklmnopqrstuvwxyz123456 echoed", got)
}

func TestSecretShield_UnknownPlaceholderIsNotRestored(t *testing.T) {
	vault := NewSecretShieldVault()
	unknown := "__S2A_SECRET_unknown_0001_OPENAI_KEY__"
	require.Equal(t, unknown, vault.RestoreString(unknown))
}
