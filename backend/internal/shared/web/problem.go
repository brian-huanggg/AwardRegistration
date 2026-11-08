package web

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// ProblemContentType is the RFC 9457 media type.
const ProblemContentType = "application/problem+json"

// FieldError is one entry of errors[].
type FieldError struct {
	Field    string    `json:"field"`
	Code     ErrorCode `json:"code"`
	Expected string    `json:"expected,omitempty"`
}

// Error is the one error type handlers and services return. The central
// adapter turns it into problem+json; a non-empty AuditReason makes the adapter
// record a track-B row (Backend.md §7). Without a reason nothing is audited.
//
// The Audit* fields are plain strings because package audit imports web.
type Error struct {
	Status int
	Code   ErrorCode
	Detail string
	Fields []FieldError
	Cause  error

	// AuditReason is the real reason of a rejected attempt (CSRF, role, state,
	// not-yours, not-found). It is stored in the row's after_json, never sent
	// to the client.
	AuditReason string
	// AuditAction is the audit action of the attempted operation, e.g.
	// "IDEA_VIEW". Required whenever AuditReason is set.
	AuditAction string
	// AuditOutcome is DENIED, FAILED or THROTTLED; empty means DENIED.
	AuditOutcome string
	// AuditTargetType and AuditTargetID name the resource, when there is one.
	AuditTargetType string
	AuditTargetID   uuid.UUID
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return string(e.Code) + ": " + e.Cause.Error()
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.Cause }

// NewError builds an *Error whose status comes from the catalogue.
func NewError(code ErrorCode, detail string) *Error {
	status, ok := Catalogue[code]
	if !ok {
		status = http.StatusInternalServerError
	}
	return &Error{Status: status, Code: code, Detail: detail}
}

type problemBody struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Code      ErrorCode    `json:"code"`
	Detail    string       `json:"detail,omitempty"`
	RequestID string       `json:"requestId,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// WriteProblem is the single problem+json writer. A 500 carries only
// INTERNAL_ERROR and the request ID: nothing from the cause leaks.
func WriteProblem(w http.ResponseWriter, r *http.Request, e *Error) {
	body := problemBody{
		Type:      "about:blank",
		Title:     http.StatusText(e.Status),
		Status:    e.Status,
		Code:      e.Code,
		Detail:    e.Detail,
		RequestID: RequestIDFrom(r.Context()),
		Errors:    e.Fields,
	}
	if e.Status >= 500 && e.Code == CodeInternalError {
		body.Detail, body.Errors = "", nil
	}
	w.Header().Set("Content-Type", ProblemContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteError converts any error into problem+json; unknown errors are 500.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if e := asError(err); e != nil {
		WriteProblem(w, r, e)
		return
	}
	WriteProblem(w, r, NewError(CodeInternalError, ""))
}
