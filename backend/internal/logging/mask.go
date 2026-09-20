package logging

import (
	"net/http"
	"strings"
)

// RedactedValue は機密値を置換するマスク文字列（design D6）。
const RedactedValue = "[REDACTED]"

// defaultSensitiveKeys は既定でマスク対象とするキー集合。すべて小文字で保持し、
// 照合時に大文字小文字を区別しない（design D6）。
var defaultSensitiveKeys = map[string]struct{}{
	"authorization": {},
	"x-api-key":     {},
	"cookie":        {},
	"set-cookie":    {},
}

// IsSensitiveKey はキーが既定の機密キー集合に含まれるかを大文字小文字非依存で判定する。
func IsSensitiveKey(key string) bool {
	_, ok := defaultSensitiveKeys[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// MaskHeader は HTTP ヘッダーの機密値を [REDACTED] へ置換した複製を返す。
// 元のヘッダーは変更しない。キー照合は大文字小文字を区別しない。
func MaskHeader(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, vals := range h {
		if IsSensitiveKey(k) {
			out[k] = []string{RedactedValue}
			continue
		}
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[k] = cp
	}
	return out
}
