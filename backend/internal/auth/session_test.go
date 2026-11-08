package auth_test

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
)

func TestBindingIsSHA256OfDecodedID(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	want := sha256.Sum256(raw)
	got, ok := auth.Binding(base64.RawURLEncoding.EncodeToString(raw))
	if !ok || string(got) != string(want[:]) {
		t.Fatalf("Binding = %x, %v; want %x", got, ok, want)
	}
}

func TestBindingRejectsMalformedIDs(t *testing.T) {
	for name, id := range map[string]string{
		"empty":      "",
		"not base64": "!!!",
		"too short":  base64.RawURLEncoding.EncodeToString(make([]byte, 31)),
		"too long":   base64.RawURLEncoding.EncodeToString(make([]byte, 33)),
		"padded":     base64.URLEncoding.EncodeToString(make([]byte, 32)),
		"std alpha":  base64.StdEncoding.EncodeToString([]byte{0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe, 0xfb, 0xff}),
	} {
		if _, ok := auth.Binding(id); ok {
			t.Errorf("%s: %q accepted", name, id)
		}
	}
}
