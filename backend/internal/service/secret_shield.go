package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const secretShieldPlaceholderPrefix = "__S2A_SECRET_"

type secretShieldPattern struct {
	label                  string
	re                     *regexp.Regexp
	valueGroup             int
	minLen                 int
	requireCredentialShape bool
}

var secretShieldPatterns = []secretShieldPattern{
	{
		label:      "JSON_SECRET",
		re:         regexp.MustCompile(`(?i)("(?:api[_-]?key|access[_-]?token|refresh[_-]?token|auth[_-]?token|password|passwd|pwd|secret|client[_-]?secret|private[_-]?key|authorization)"\s*:\s*")([^"\\\r\n]+)(")`),
		valueGroup: 2,
		minLen:     4,
	},
	{
		label:                  "BEARER",
		re:                     regexp.MustCompile(`(?i)(\bAuthorization\s*[:=]\s*Bearer\s+)([A-Za-z0-9._~+/-]+)`),
		valueGroup:             2,
		minLen:                 16,
		requireCredentialShape: true,
	},
	{
		label:                  "ASSIGNMENT_SECRET",
		re:                     regexp.MustCompile(`(?i)(\b(?:[A-Z0-9_]*API[_-]?KEY|API[_-]?KEY|ACCESS[_-]?TOKEN|REFRESH[_-]?TOKEN|AUTH[_-]?TOKEN|PASSWORD|PASSWD|PWD|SECRET|CLIENT[_-]?SECRET|PRIVATE[_-]?KEY)\b\s*[:=]\s*["']?)([A-Za-z0-9._~+/=@$%:;!#-]+)(["']?)`),
		valueGroup:             2,
		minLen:                 4,
		requireCredentialShape: true,
	},
	{
		label:      "OPENAI_KEY",
		re:         regexp.MustCompile(`\bsk-(?:proj-|ant-)?[A-Za-z0-9_-]{20,}\b`),
		valueGroup: 0,
		minLen:     23,
	},
	{
		label:      "GITHUB_TOKEN",
		re:         regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{30,}\b`),
		valueGroup: 0,
		minLen:     34,
	},
	{
		label:      "SLACK_TOKEN",
		re:         regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{20,}\b`),
		valueGroup: 0,
		minLen:     25,
	},
	{
		label:      "JWT",
		re:         regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`),
		valueGroup: 0,
		minLen:     32,
	},
	{
		label:      "PRIVATE_KEY",
		re:         regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
		valueGroup: 0,
		minLen:     64,
	},
}

var (
	uuidLikeSecretShieldPattern      = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	shellEnvSecretShieldPattern      = regexp.MustCompile(`^\$[A-Za-z_][A-Za-z0-9_]*$`)
	templateEnvSecretShieldPattern   = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*(?::[-?][^}]*)?\}$`)
	bracketEnvSecretShieldPattern    = regexp.MustCompile(`(?i)^(?:os\.environ|getenv)\[["'][A-Za-z_][A-Za-z0-9_]*["']\]$`)
	knownCodeRefSecretShieldPrefixes = []string{
		"process.env.",
		"import.meta.env.",
		"os.environ.",
		"env.",
		"config.",
		"settings.",
	}
)

// SecretShieldVault stores a request-local placeholder mapping. It must not be
// persisted or shared across requests because it contains the original secrets.
type SecretShieldVault struct {
	mu                  sync.RWMutex
	nonce               string
	next                int
	placeholderToSecret map[string]string
	secretToPlaceholder map[string]string
	replacer            *strings.Replacer
	maxPlaceholderLen   int
}

func NewSecretShieldVault() *SecretShieldVault {
	return &SecretShieldVault{
		nonce:               newSecretShieldNonce(),
		placeholderToSecret: make(map[string]string),
		secretToPlaceholder: make(map[string]string),
	}
}

func (v *SecretShieldVault) Empty() bool {
	if v == nil {
		return true
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.placeholderToSecret) == 0
}

func (v *SecretShieldVault) ProtectBytes(body []byte) ([]byte, bool) {
	if v == nil || len(body) == 0 {
		return body, false
	}
	protected := v.ProtectString(string(body))
	if protected == string(body) {
		return body, false
	}
	return []byte(protected), true
}

func (v *SecretShieldVault) ProtectString(input string) string {
	if v == nil || input == "" {
		return input
	}
	out := input
	for _, pattern := range secretShieldPatterns {
		out = v.applyPattern(out, pattern)
	}
	return out
}

func (v *SecretShieldVault) RestoreBytes(data []byte) []byte {
	if v == nil || len(data) == 0 || v.Empty() {
		return data
	}
	return []byte(v.RestoreString(string(data)))
}

func (v *SecretShieldVault) RestoreString(input string) string {
	if v == nil || input == "" {
		return input
	}
	v.mu.RLock()
	replacer := v.replacer
	v.mu.RUnlock()
	if replacer == nil {
		return input
	}
	return replacer.Replace(input)
}

func (v *SecretShieldVault) PlaceholderCount() int {
	if v == nil {
		return 0
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.placeholderToSecret)
}

func (v *SecretShieldVault) MaxPlaceholderLen() int {
	if v == nil {
		return 0
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.maxPlaceholderLen
}

func (v *SecretShieldVault) NewStreamRestorer() *SecretShieldStreamRestorer {
	return &SecretShieldStreamRestorer{vault: v}
}

func (v *SecretShieldVault) applyPattern(input string, pattern secretShieldPattern) string {
	matches := pattern.re.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input
	}
	var b strings.Builder
	last := 0
	changed := false
	for _, match := range matches {
		groupStart := pattern.valueGroup * 2
		groupEnd := groupStart + 1
		if groupEnd >= len(match) || match[groupStart] < 0 || match[groupEnd] < 0 {
			continue
		}
		start, end := match[groupStart], match[groupEnd]
		if start < last || end < start {
			continue
		}
		candidate := input[start:end]
		if !isSecretShieldCandidate(candidate, pattern.minLen, pattern.requireCredentialShape) {
			continue
		}
		placeholder := v.placeholderFor(candidate, pattern.label)
		if !changed {
			b.Grow(len(input))
			changed = true
		}
		b.WriteString(input[last:start])
		b.WriteString(placeholder)
		last = end
	}
	if !changed {
		return input
	}
	b.WriteString(input[last:])
	return b.String()
}

func (v *SecretShieldVault) placeholderFor(secret string, label string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	if existing := v.secretToPlaceholder[secret]; existing != "" {
		return existing
	}
	v.next++
	placeholder := fmt.Sprintf("%s%s_%04d_%s__", secretShieldPlaceholderPrefix, v.nonce, v.next, sanitizeSecretShieldLabel(label))
	v.secretToPlaceholder[secret] = placeholder
	v.placeholderToSecret[placeholder] = secret
	v.rebuildReplacerLocked()
	if len(placeholder) > v.maxPlaceholderLen {
		v.maxPlaceholderLen = len(placeholder)
	}
	return placeholder
}

func (v *SecretShieldVault) rebuildReplacerLocked() {
	placeholders := make([]string, 0, len(v.placeholderToSecret))
	for placeholder := range v.placeholderToSecret {
		placeholders = append(placeholders, placeholder)
	}
	// Prefer the longest placeholders first. This is defensive if a future label
	// format accidentally makes one placeholder a prefix of another.
	sort.Slice(placeholders, func(i, j int) bool {
		return len(placeholders[i]) > len(placeholders[j])
	})
	pairs := make([]string, 0, len(placeholders)*2)
	for _, placeholder := range placeholders {
		pairs = append(pairs, placeholder, v.placeholderToSecret[placeholder])
	}
	v.replacer = strings.NewReplacer(pairs...)
}

func (v *SecretShieldVault) longestPlaceholderPrefixSuffix(input string) int {
	if v == nil || input == "" {
		return 0
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	max := 0
	for placeholder := range v.placeholderToSecret {
		limit := len(placeholder) - 1 // complete placeholders can be restored now.
		if limit > len(input) {
			limit = len(input)
		}
		for n := 1; n <= limit; n++ {
			if n <= max {
				continue
			}
			if strings.HasSuffix(input, placeholder[:n]) {
				max = n
			}
		}
	}
	return max
}

// SecretShieldStreamRestorer restores placeholders in a chunked stream without
// leaking split placeholders. It only buffers a suffix that is a proper prefix of
// a known placeholder, so normal SSE flushes are not delayed.
type SecretShieldStreamRestorer struct {
	vault *SecretShieldVault
	carry string
}

func (r *SecretShieldStreamRestorer) RestoreChunk(data []byte, final bool) []byte {
	if r == nil || r.vault == nil || r.vault.Empty() {
		return data
	}
	combined := r.carry + string(data)
	r.carry = ""
	if combined == "" {
		return nil
	}
	if final {
		return []byte(r.vault.RestoreString(combined))
	}
	hold := r.vault.longestPlaceholderPrefixSuffix(combined)
	if hold > 0 {
		r.carry = combined[len(combined)-hold:]
		combined = combined[:len(combined)-hold]
	}
	if combined == "" {
		return nil
	}
	return []byte(r.vault.RestoreString(combined))
}

func (r *SecretShieldStreamRestorer) BufferedLen() int {
	if r == nil {
		return 0
	}
	return len(r.carry)
}

func (r *SecretShieldStreamRestorer) Finalize() []byte {
	if r == nil {
		return nil
	}
	return r.RestoreChunk(nil, true)
}

func isSecretShieldCandidate(value string, minLen int, requireCredentialShape bool) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < minLen {
		return false
	}
	if strings.HasPrefix(trimmed, secretShieldPlaceholderPrefix) {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, marker := range []string{"your_api_key", "your-api-key", "your-token", "your_token", "example", "placeholder", "changeme", "change_me", "dummy"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	if uuidLikeSecretShieldPattern.MatchString(trimmed) {
		return false
	}
	if looksLikeSecretShieldCodeReference(trimmed) {
		return false
	}
	if allSameRune(trimmed) {
		return false
	}
	if requireCredentialShape {
		return hasCredentialShape(trimmed)
	}
	return true
}

func looksLikeSecretShieldCodeReference(value string) bool {
	trimmed := strings.Trim(strings.TrimSpace(value), `"'`)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range knownCodeRefSecretShieldPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return shellEnvSecretShieldPattern.MatchString(trimmed) ||
		templateEnvSecretShieldPattern.MatchString(trimmed) ||
		bracketEnvSecretShieldPattern.MatchString(trimmed)
}

func hasCredentialShape(value string) bool {
	hasLetter := false
	hasDigit := false
	hasSymbol := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	if len(value) >= 12 {
		return hasDigit || hasSymbol
	}
	return hasLetter && hasDigit
}

func allSameRune(value string) bool {
	var first rune
	seen := false
	for _, r := range value {
		if !seen {
			first = r
			seen = true
			continue
		}
		if r != first {
			return false
		}
	}
	return seen
}

func sanitizeSecretShieldLabel(label string) string {
	label = strings.ToUpper(strings.TrimSpace(label))
	if label == "" {
		return "SECRET"
	}
	var b strings.Builder
	for _, r := range label {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "SECRET"
	}
	return b.String()
}

func newSecretShieldNonce() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
