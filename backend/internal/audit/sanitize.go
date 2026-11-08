package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Column limits (V002). User-controlled text is cut to these by code point, so
// a hostile header can never make the audit insert fail.
const (
	maxEmpNo     = 64
	maxMethod    = 8
	maxRoute     = 200
	maxErrorCode = 64
	maxUserAgent = 256 // PRD §6.11

	// maxJSONBytes caps before_json / after_json; a bigger document is replaced
	// by a marker, never cut in the middle (that would be invalid JSON).
	maxJSONBytes = 16 << 10
)

// redacted replaces the value of a secret-looking key.
const redacted = "[REDACTED]"

// secretKeys are JSON keys whose values are never stored (PRD §6.11: no
// passwords, tokens, session IDs, CSRF values or file contents). Compared after
// lower-casing and removing "_" and "-".
var secretKeys = map[string]bool{
	"password": true, "passwd": true, "newpassword": true, "oldpassword": true, "currentpassword": true,
	"token": true, "csrf": true, "csrftoken": true, "xcsrftoken": true,
	"session": true, "sessionid": true, "cookie": true, "setcookie": true,
	"authorization": true, "secret": true, "content": true, "filecontent": true,
}

// clean makes s safe to store and cuts it to max code points: invalid UTF-8
// and control characters (including NUL and line breaks, which would forge log
// lines) are dropped, and the result is trimmed.
func clean(s string, max int) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return s
}

// cleanJSON validates raw, redacts secret keys and returns the compact
// document; nil means "store NULL" (empty input or JSON null).
func cleanJSON(raw json.RawMessage) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // keep numbers exactly as written
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("audit: invalid JSON: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("audit: trailing data after JSON value")
	}
	if v == nil {
		return nil, nil
	}
	out, err := json.Marshal(redact(v))
	if err != nil {
		return nil, fmt.Errorf("audit: marshal JSON: %w", err)
	}
	if len(out) > maxJSONBytes {
		return json.Marshal(map[string]any{"truncated": true, "bytes": len(out)})
	}
	return out, nil
}

func redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			norm := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(k))
			if secretKeys[norm] {
				t[k] = redacted
			} else {
				t[k] = redact(val)
			}
		}
	case []any:
		for i, val := range t {
			t[i] = redact(val)
		}
	}
	return v
}
