package account

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/text"
)

// Reasons an Admin entry is rejected. They are log/metric labels, never user
// text, and never contain the hash.
const (
	RejectEmpNo     = "INVALID_EMP_NO"
	RejectName      = "INVALID_NAME"
	RejectHash      = "INVALID_HASH"
	RejectCost      = "BCRYPT_COST" // cost != 10 (PRD §4.2, OI-13)
	RejectDuplicate = "DUPLICATE_EMP_NO"
)

// AdminEntry is one Admin as the directory holds it (PRD §4.2): no email, no
// password policy. Hash is a bcrypt hash of cost 10, already validated.
type AdminEntry struct {
	EmpNo string
	Name  string
	Hash  string
}

// RejectedEntry is an entry the directory refused. The rest of the snapshot
// is still valid (PRD §4.2: "entry skipped").
type RejectedEntry struct {
	EmpNo  string // as read; may be malformed or empty
	Reason string // one of the Reject* constants
}

// AdminSnapshot is one complete read of the directory.
type AdminSnapshot struct {
	Entries  []AdminEntry
	Rejected []RejectedEntry
}

// AdminDirectory is the source of Admin accounts. The Vault implementation
// (B2-06) and the file implementation (local and test profiles only) satisfy
// it. List returns an error only when the directory as a whole cannot be read
// or parsed; a bad single entry goes to Snapshot.Rejected so AdminSync can
// keep the other Admins (OI-40).
type AdminDirectory interface {
	List(ctx context.Context) (AdminSnapshot, error)
}

// FileAdminDirectory reads a JSON file on every List call:
//
//	[{"empNo":"900001","name":"Ada","hash":"$2a$10$..."}]
//
// For the local and test profiles only; it is never wired in k3s or dev.
type FileAdminDirectory struct{ Path string }

type adminRecord struct {
	EmpNo *string `json:"empNo"`
	Name  *string `json:"name"`
	Hash  *string `json:"hash"`
}

// List implements AdminDirectory.
func (d FileAdminDirectory) List(ctx context.Context) (AdminSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return AdminSnapshot{}, err
	}
	b, err := os.ReadFile(d.Path)
	if err != nil {
		return AdminSnapshot{}, fmt.Errorf("admin directory: %w", err)
	}
	var recs []adminRecord
	if err := json.Unmarshal(b, &recs); err != nil {
		return AdminSnapshot{}, fmt.Errorf("admin directory: parse %s: %w", d.Path, err)
	}
	return validateAdminRecords(recs), nil
}

// validateAdminRecords is shared with the Vault implementation so both apply
// the same entry rules, in this order: employee ID, name, hash, cost,
// duplicate. The first duplicate wins.
func validateAdminRecords(recs []adminRecord) AdminSnapshot {
	var snap AdminSnapshot
	seen := make(map[string]struct{}, len(recs))
	for _, r := range recs {
		empNo := deref(r.EmpNo)
		reject := func(reason string) {
			snap.Rejected = append(snap.Rejected, RejectedEntry{EmpNo: empNo, Reason: reason})
		}
		switch {
		case checkEmpNo(empNo) != text.None:
			reject(RejectEmpNo)
			continue
		case text.Trim(deref(r.Name)) == "":
			reject(RejectName)
			continue
		}
		hash := deref(r.Hash)
		cost, err := bcrypt.Cost([]byte(hash))
		if err != nil {
			reject(RejectHash)
			continue
		}
		if cost != bcryptCost {
			reject(RejectCost)
			continue
		}
		if _, dup := seen[empNo]; dup {
			reject(RejectDuplicate)
			continue
		}
		seen[empNo] = struct{}{}
		snap.Entries = append(snap.Entries, AdminEntry{EmpNo: empNo, Name: text.Trim(deref(r.Name)), Hash: hash})
	}
	return snap
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
