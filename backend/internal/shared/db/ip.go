package db

import (
	"database/sql/driver"
	"fmt"
	"net/netip"
)

// IP is a MariaDB INET6 value. netip.Addr has no sql.Scanner, so this wraps it.
type IP struct{ netip.Addr }

// Scan implements sql.Scanner. MariaDB returns INET6 as text.
func (ip *IP) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("db.IP: cannot scan %T", src)
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return fmt.Errorf("db.IP: %w", err)
	}
	ip.Addr = a
	return nil
}

// Value implements driver.Valuer. An invalid (zero) address is an error; use
// NullIP for nullable columns.
func (ip IP) Value() (driver.Value, error) {
	if !ip.IsValid() {
		return nil, fmt.Errorf("db.IP: invalid address")
	}
	return ip.String(), nil
}

// NullIP is a nullable INET6 column.
type NullIP struct {
	IP    IP
	Valid bool
}

// Scan implements sql.Scanner.
func (n *NullIP) Scan(src any) error {
	if src == nil {
		n.IP, n.Valid = IP{}, false
		return nil
	}
	if err := n.IP.Scan(src); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// Value implements driver.Valuer.
func (n NullIP) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.IP.Value()
}
