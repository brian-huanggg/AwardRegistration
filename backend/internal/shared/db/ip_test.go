package db

import (
	"net/netip"
	"testing"
)

func TestIPScanValue(t *testing.T) {
	for _, in := range []any{"2001:db8::1", []byte("2001:db8::1")} {
		var ip IP
		if err := ip.Scan(in); err != nil {
			t.Fatal(err)
		}
		if ip.Addr != netip.MustParseAddr("2001:db8::1") {
			t.Errorf("scanned %v", ip.Addr)
		}
		v, err := ip.Value()
		if err != nil || v != "2001:db8::1" {
			t.Errorf("Value = %v, %v", v, err)
		}
	}
	var v4 IP
	if err := v4.Scan("192.0.2.7"); err != nil || v4.String() != "192.0.2.7" {
		t.Errorf("ipv4: %v %v", v4, err)
	}
}

func TestIPErrors(t *testing.T) {
	var ip IP
	if ip.Scan("nope") == nil || ip.Scan(42) == nil {
		t.Error("bad input must fail")
	}
	if _, err := (IP{}).Value(); err == nil {
		t.Error("zero IP must not be written")
	}
}

func TestNullIP(t *testing.T) {
	var n NullIP
	if err := n.Scan(nil); err != nil || n.Valid {
		t.Fatalf("nil: %+v %v", n, err)
	}
	if v, err := n.Value(); v != nil || err != nil {
		t.Errorf("null Value = %v, %v", v, err)
	}
	if err := n.Scan("::1"); err != nil || !n.Valid {
		t.Fatalf("::1: %+v %v", n, err)
	}
	if v, _ := n.Value(); v != "::1" {
		t.Errorf("Value = %v", v)
	}
	if n.Scan("bad") == nil {
		t.Error("bad input must fail")
	}
	if err := n.Scan(nil); err != nil || n.Valid {
		t.Error("rescan nil must reset")
	}
}
