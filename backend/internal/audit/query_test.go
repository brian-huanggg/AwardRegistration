package audit_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func ptr[T any](v T) *T { return &v }

func TestCursorRoundTrip(t *testing.T) {
	c := audit.Cursor{OccurredAt: time.Date(2026, 11, 9, 1, 2, 3, 456789000, time.UTC), ID: 42}
	got, ok := audit.DecodeCursor(c.Encode())
	if !ok || !got.OccurredAt.Equal(c.OccurredAt) || got.ID != 42 {
		t.Fatalf("round trip = %+v, %v", got, ok)
	}
}

func TestDecodeCursorRejects(t *testing.T) {
	enc := b64
	for name, in := range map[string]string{
		"empty":         "",
		"not base64":    "!!!",
		"padded":        b64("1,2") + "=",
		"no comma":      enc("12"),
		"negative time": enc("-1,2"),
		"huge time":     enc("999999999999999999,2"),
		"bad id":        enc("1,x"),
		"negative id":   enc("1,-2"),
		"plus sign":     enc("+1,2"),
		"leading zero":  enc("01,2"),
		"extra field":   enc("1,2,3"),
	} {
		if _, ok := audit.DecodeCursor(in); ok {
			t.Errorf("%s: %q accepted", name, in)
		}
	}
}

func TestParseQuery(t *testing.T) {
	uid := "5f0f4a7e-6a53-4a6c-9f3a-0d6a1b2c3d4e"
	q, errs := audit.ParseQuery(api.ListAuditLogsParams{
		From: ptr("2026-01-01T08:00:00+08:00"), To: ptr("2026-02-01T00:00:00Z"),
		ActorEmpNo: ptr("100001"), Action: ptr([]string{"IDEA_VIEW", "AUDIT_QUERY", "IDEA_VIEW"}),
		Outcome: ptr("DENIED"), TargetType: ptr("IDEA"), TargetId: &uid, RequestId: &uid,
		Ip: ptr("::ffff:10.0.0.1"), Limit: ptr("100"),
	})
	if len(errs) != 0 {
		t.Fatalf("errs = %+v", errs)
	}
	if !q.From.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) || q.To == nil || len(q.Actions) != 2 ||
		q.Limit != 100 || q.IP.String() != "10.0.0.1" || q.Outcome != audit.Denied {
		t.Fatalf("q = %+v", q)
	}
	if d, _ := audit.ParseQuery(api.ListAuditLogsParams{}); d.Limit != audit.DefaultLimit {
		t.Fatalf("default limit = %d", d.Limit)
	}
}

func TestParseQueryErrors(t *testing.T) {
	_, errs := audit.ParseQuery(api.ListAuditLogsParams{
		From: ptr("2026-01-01"), To: ptr("nope"), ActorEmpNo: ptr(strings.Repeat("9", 65)),
		Action: ptr([]string{"IDEA_VIEW", "NOPE"}), Outcome: ptr("x"), TargetType: ptr("x"),
		TargetId: ptr("not-a-uuid"), RequestId: ptr("00000000-0000-0000-0000-000000000000"),
		Ip: ptr("fe80::1%eth0"), Cursor: ptr("garbage"), Limit: ptr("101"),
	})
	want := []struct {
		field string
		code  web.ErrorCode
	}{
		{"from", web.CodeInvalidFormat}, {"to", web.CodeInvalidFormat}, {"actorEmpNo", web.CodeInvalidFormat},
		{"action", web.CodeInvalidEnum}, {"outcome", web.CodeInvalidEnum}, {"targetType", web.CodeInvalidEnum},
		{"targetId", web.CodeInvalidFormat}, {"requestId", web.CodeInvalidFormat}, {"ip", web.CodeInvalidFormat},
		{"cursor", web.CodeInvalidFormat}, {"limit", web.CodeOutOfRange},
	}
	if len(errs) != len(want) {
		t.Fatalf("errs = %+v", errs)
	}
	for i, w := range want {
		if errs[i].Field != w.field || errs[i].Code != w.code {
			t.Errorf("errs[%d] = %+v, want %+v", i, errs[i], w)
		}
	}
	for _, l := range []string{"0", "-1", "x", ""} {
		if _, e := audit.ParseQuery(api.ListAuditLogsParams{Limit: &l}); len(e) != 1 || e[0].Field != "limit" {
			t.Errorf("limit %q: %+v", l, e)
		}
	}
}

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
