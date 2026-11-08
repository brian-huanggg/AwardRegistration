// Package account owns app_user: registration today; profile, password change
// and admin sync land with their own PRs. Backend.md §2.
package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/text"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// bcryptCost is the only cost the table accepts (PRD §6.4, OI-13).
const bcryptCost = 10

// Recorder writes the success audit row in the registration transaction.
// *audit.Service implements it.
type Recorder interface {
	Record(ctx context.Context, tx *sql.Tx, e audit.Event) error
}

// Service implements the account use cases.
type Service struct {
	DB     *sql.DB
	Audit  Recorder
	Policy PasswordPolicy
}

// RegisterCommand is the raw, unvalidated input.
type RegisterCommand struct {
	EmpNo, Name, Email, Password string
}

// Request is the transport facts the audit row needs.
type Request struct {
	Method, Route, UserAgent string
	IP                       netip.Addr
}

// Registered is the stored account (never the hash).
type Registered struct {
	ID                 uuid.UUID
	EmpNo, Name, Email string
}

// Register creates a USER. Order (PRD §6.3): format → password policy →
// uniqueness. Every failure is a *web.Error carrying the AUTH_REGISTER FAILED
// audit fields, so the central adapter writes the track-B row; a success
// writes its SUCCESS row in the same transaction as the INSERT (fail-closed).
func (s *Service) Register(ctx context.Context, cmd RegisterCommand, req Request) (Registered, error) {
	empNoOK := checkEmpNo(cmd.EmpNo) == text.None

	var fields []web.FieldError
	add := func(field string, p text.Problem) {
		if p != text.None {
			fields = append(fields, web.FieldError{Field: field, Code: web.ErrorCode(p)})
		}
	}
	add(fieldEmpNo, checkEmpNo(cmd.EmpNo))
	name, p := text.CheckName(cmd.Name, maxNameLen)
	add(fieldName, p)
	email, p := checkEmail(cmd.Email)
	add(fieldEmail, p)

	policyEmpNo := ""
	if empNoOK {
		policyEmpNo = cmd.EmpNo
	}
	fields = append(fields, s.Policy.Check(cmd.Password, policyEmpNo)...)

	if len(fields) > 0 {
		e := web.NewError(topCode(fields), "the request has invalid fields")
		e.Fields = fields
		return Registered{}, failed(e, "validation", cmd.EmpNo, empNoOK)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcryptCost)
	if err != nil {
		return Registered{}, fmt.Errorf("hash password: %w", err)
	}

	u := Registered{ID: uuid.New(), EmpNo: cmd.EmpNo, Name: name, Email: email}
	after, _ := json.Marshal(map[string]string{"empNo": u.EmpNo, "name": u.Name}) // no email (PRD §6.11)

	_, err = db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		if err := store.InsertUser(ctx, tx, store.NewUser{
			ID: u.ID, EmpNo: u.EmpNo, Name: u.Name, Email: u.Email, PasswordHash: string(hash),
		}); err != nil {
			return err
		}
		return s.Audit.Record(ctx, tx, audit.Event{
			Action: audit.AuthRegister, Outcome: audit.Success,
			TargetType: audit.TargetUser, TargetID: u.ID,
			Method: req.Method, Route: req.Route, Status: http.StatusCreated,
			IP: req.IP, UserAgent: req.UserAgent,
			After: after,
		})
	})
	if err != nil {
		if e := duplicate(err); e != nil {
			return Registered{}, failed(e, e.AuditReason, cmd.EmpNo, true)
		}
		return Registered{}, err
	}
	return u, nil
}

// duplicate maps a unique-key violation to 409. emp_no is checked first by the
// server (it is the first unique key), so a row clashing on both says EMP_NO_TAKEN.
func duplicate(err error) *web.Error {
	key, ok := db.DuplicateKey(err)
	if !ok {
		return nil
	}
	var e *web.Error
	switch {
	case strings.Contains(key, "emp_no"):
		e = web.NewError(web.CodeEmpNoTaken, "employee ID is already registered")
		e.AuditReason = "emp_no_taken"
	case strings.Contains(key, "email"):
		e = web.NewError(web.CodeEmailTaken, "email is already registered")
		e.AuditReason = "email_taken"
	default:
		return nil
	}
	e.Cause = err
	return e
}

// failed marks e as an AUTH_REGISTER FAILED attempt (PRD §6.11): the typed
// employee ID goes into the dedicated column (sanitized by audit), and into
// after_json only when well-formed.
func failed(e *web.Error, reason, empNo string, wellFormed bool) *web.Error {
	e.AuditReason = reason
	e.AuditAction = string(audit.AuthRegister)
	e.AuditOutcome = string(audit.Failed)
	e.AuditSubmittedEmpNo = empNo
	if wellFormed {
		e.AuditDetail = map[string]string{"empNo": empNo}
	}
	return e
}
