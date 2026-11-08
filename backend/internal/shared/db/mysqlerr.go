package db

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

// MySQL/MariaDB server error numbers that InTx treats specially.
const (
	errLockWaitTimeout = 1205 // ER_LOCK_WAIT_TIMEOUT
	errDeadlock        = 1213 // ER_LOCK_DEADLOCK
)

// IsContention reports whether err is a lock-wait timeout (1205) or a
// deadlock (1213). Both map to 503 SERVICE_UNAVAILABLE and a metric.
func IsContention(err error) bool {
	var me *mysql.MySQLError
	if !errors.As(err, &me) {
		return false
	}
	return me.Number == errLockWaitTimeout || me.Number == errDeadlock
}

// sqlStateClass returns the two-character SQLState class of a server error,
// or "" when err is not a *mysql.MySQLError.
func sqlStateClass(err error) string {
	var me *mysql.MySQLError
	if !errors.As(err, &me) {
		return ""
	}
	return string(me.SQLState[:2])
}
