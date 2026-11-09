package db

import (
	"errors"
	"strings"

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

const errDupEntry = 1062 // ER_DUP_ENTRY

// DuplicateKey reports whether err is a unique-key violation (1062) and names
// the violated key, as in "Duplicate entry 'x' for key 'uq_app_user_email'".
func DuplicateKey(err error) (key string, ok bool) {
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != errDupEntry {
		return "", false
	}
	const marker = "for key '"
	i := strings.LastIndex(me.Message, marker)
	if i < 0 {
		return "", true
	}
	rest := me.Message[i+len(marker):]
	if j := strings.IndexByte(rest, '\''); j >= 0 {
		rest = rest[:j]
	}
	// MariaDB may qualify the key with the table name ("app_user.uq_x").
	if k := strings.LastIndexByte(rest, '.'); k >= 0 {
		rest = rest[k+1:]
	}
	return rest, true
}

// ER_CONSTRAINT_FAILED (MariaDB) and ER_CHECK_CONSTRAINT_VIOLATED (MySQL).
const (
	errConstraintFailed = 4025
	errCheckViolated    = 3819
)

// IsCheckViolation reports whether err is a CHECK constraint failure: the
// table refused a value the application did not catch.
func IsCheckViolation(err error) bool {
	var me *mysql.MySQLError
	if !errors.As(err, &me) {
		return false
	}
	return me.Number == errConstraintFailed || me.Number == errCheckViolated
}
