package auth_test

// Fuzz targets for the auth parsers and signers (security testing layer 3).
// Seeds run on every `go test`; CI fuzzes each target briefly with
// `make fuzz` and a corpus entry under testdata/fuzz is a regression test.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
)

// LockKey must be bounded, free of control characters and idempotent: a
// non-idempotent key would let variants of one ID count against different
// lock rows.
func FuzzLockKey(f *testing.F) {
	for _, s := range []string{"123456", "12345\x00", "\t123456\n", strings.Repeat("é", 40), "\xff\xfe", "123456\u0085", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		k := auth.LockKey(s)
		if n := utf8.RuneCountInString(k); n > 32 {
			t.Fatalf("key has %d runes, max 32: %q", n, k)
		}
		if strings.ContainsFunc(k, unicode.IsControl) {
			t.Fatalf("control character kept: %q", k)
		}
		if again := auth.LockKey(k); again != k {
			t.Fatalf("not idempotent: LockKey(%q)=%q, LockKey of that=%q", s, k, again)
		}
	})
}

// No input string may verify for a binding it was not issued for (forgery,
// cross-session replay), and an issued token must verify for its own binding.
func FuzzCSRFVerifyNoForgery(f *testing.F) {
	c, err := auth.NewCSRF(strings.Repeat("k", 32), strings.Repeat("p", 32))
	if err != nil {
		f.Fatal(err)
	}
	attacker := append([]byte{0x01}, bytes.Repeat([]byte{2}, 32)...)
	victim := append([]byte{0x01}, bytes.Repeat([]byte{1}, 32)...)
	tok, err := c.Issue(attacker)
	if err != nil {
		f.Fatal(err)
	}
	nonce, sig, _ := strings.Cut(tok, ".")
	for _, s := range []string{tok, nonce + ".", "." + sig, nonce + sig, tok + "=", tok + ".x", "", ".", strings.Repeat("A", 22) + "." + strings.Repeat("A", 43)} {
		f.Add(s, []byte{})
	}
	f.Fuzz(func(t *testing.T, s string, binding []byte) {
		if c.Verify(s, victim) {
			t.Fatalf("token never issued for the victim verified: %q", s)
		}
		if bytes.Equal(binding, attacker) {
			return
		}
		if c.Verify(tok, binding) {
			t.Fatalf("attacker's token verified for binding %x", binding)
		}
		own, err := c.Issue(binding)
		if err != nil {
			t.Fatal(err)
		}
		if !c.Verify(own, binding) {
			t.Fatalf("issued token does not verify for its binding %x", binding)
		}
	})
}

// Binding (the session ID parser) must accept only values that decode to
// exactly 32 bytes, return their SHA-256, and never panic on cookie input.
func FuzzSessionBinding(f *testing.F) {
	valid := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	for _, s := range []string{valid, valid + "=", valid[:42], valid + "A", "", "' OR '1'='1", strings.Repeat("A", 4096), "+/+/"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		b, ok := auth.Binding(s)
		if !ok {
			if b != nil {
				t.Fatalf("rejected %q but returned a binding", s)
			}
			return
		}
		raw, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil || len(raw) != 32 {
			t.Fatalf("accepted %q: decodes to %d bytes, err %v", s, len(raw), err)
		}
		if sum := sha256.Sum256(raw); !bytes.Equal(b, sum[:]) {
			t.Fatalf("binding of %q is not SHA-256 of the ID", s)
		}
	})
}
