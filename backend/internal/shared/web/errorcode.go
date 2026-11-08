// Package web holds the HTTP building blocks shared by every module:
// request ID, recover, access log, problem+json and transport-level errors.
package web

// ErrorCode is the stable, machine-readable contract of an error response
// (docs/API/error-codes.md). The catalogue below must equal
// testdata/error-codes.json; a test locks that.
type ErrorCode string

// Error codes, grouped by HTTP status.
const (
	CodeMalformedRequest    ErrorCode = "MALFORMED_REQUEST"
	CodeUnauthenticated     ErrorCode = "UNAUTHENTICATED"
	CodeInvalidCredentials  ErrorCode = "INVALID_CREDENTIALS" //nolint:gosec // error code name, not a credential
	CodeCSRFInvalid         ErrorCode = "CSRF_INVALID"
	CodeForbidden           ErrorCode = "FORBIDDEN"
	CodeSubmissionNotOpen   ErrorCode = "SUBMISSION_NOT_OPEN"
	CodeSubmissionClosed    ErrorCode = "SUBMISSION_CLOSED"
	CodeNotFound            ErrorCode = "NOT_FOUND"
	CodeMethodNotAllowed    ErrorCode = "METHOD_NOT_ALLOWED"
	CodeNotAcceptable       ErrorCode = "NOT_ACCEPTABLE"
	CodeEmpNoTaken          ErrorCode = "EMP_NO_TAKEN"
	CodeEmailTaken          ErrorCode = "EMAIL_TAKEN"
	CodeIdeaDuplicate       ErrorCode = "IDEA_DUPLICATE"
	CodeAlreadyMember       ErrorCode = "ALREADY_MEMBER"
	CodeJoinRequestExists   ErrorCode = "JOIN_REQUEST_EXISTS"
	CodeTeamFull            ErrorCode = "TEAM_FULL"
	CodeJoinRequestNotPend  ErrorCode = "JOIN_REQUEST_NOT_PENDING"
	CodeVersionConflict     ErrorCode = "VERSION_CONFLICT"
	CodeFileTooLarge        ErrorCode = "FILE_TOO_LARGE"
	CodeRequestTooLarge     ErrorCode = "REQUEST_TOO_LARGE"
	CodeUnsupportedFileType ErrorCode = "UNSUPPORTED_FILE_TYPE"
	CodeUnsupportedMedia    ErrorCode = "UNSUPPORTED_MEDIA_TYPE"
	CodeValidationFailed    ErrorCode = "VALIDATION_FAILED"
	CodePasswordPolicy      ErrorCode = "PASSWORD_POLICY"
	CodeInvalidNameChars    ErrorCode = "INVALID_NAME_CHARS"
	CodeFilenameTooLong     ErrorCode = "FILENAME_TOO_LONG"
	CodeFilenameMismatch    ErrorCode = "FILENAME_MISMATCH"
	CodeEmptyFile           ErrorCode = "EMPTY_FILE"
	CodeCannotJoinOwnIdea   ErrorCode = "CANNOT_JOIN_OWN_IDEA"
	CodeExportTooLarge      ErrorCode = "EXPORT_TOO_LARGE"
	CodeLoginThrottled      ErrorCode = "LOGIN_THROTTLED"
	CodeRateLimited         ErrorCode = "RATE_LIMITED"
	CodeInternalError       ErrorCode = "INTERNAL_ERROR"
	CodeStorageUnavailable  ErrorCode = "STORAGE_UNAVAILABLE"
	CodeServiceUnavailable  ErrorCode = "SERVICE_UNAVAILABLE"
	CodeInvalidFormat       ErrorCode = "INVALID_FORMAT"
	CodeRequired            ErrorCode = "REQUIRED"
	CodeOutOfRange          ErrorCode = "OUT_OF_RANGE"
	CodeInvalidEnum         ErrorCode = "INVALID_ENUM"
)

// Catalogue maps each code to its HTTP status. The generic per-field codes
// (REQUIRED, INVALID_FORMAT, OUT_OF_RANGE, INVALID_ENUM) appear inside errors[]
// of a 422 response.
var Catalogue = map[ErrorCode]int{
	CodeMalformedRequest:    400,
	CodeUnauthenticated:     401,
	CodeInvalidCredentials:  401,
	CodeCSRFInvalid:         403,
	CodeForbidden:           403,
	CodeSubmissionNotOpen:   403,
	CodeSubmissionClosed:    403,
	CodeNotFound:            404,
	CodeMethodNotAllowed:    405,
	CodeNotAcceptable:       406,
	CodeEmpNoTaken:          409,
	CodeEmailTaken:          409,
	CodeIdeaDuplicate:       409,
	CodeAlreadyMember:       409,
	CodeJoinRequestExists:   409,
	CodeTeamFull:            409,
	CodeJoinRequestNotPend:  409,
	CodeVersionConflict:     412,
	CodeFileTooLarge:        413,
	CodeRequestTooLarge:     413,
	CodeUnsupportedFileType: 415,
	CodeUnsupportedMedia:    415,
	CodeValidationFailed:    422,
	CodePasswordPolicy:      422,
	CodeInvalidNameChars:    422,
	CodeFilenameTooLong:     422,
	CodeFilenameMismatch:    422,
	CodeEmptyFile:           422,
	CodeCannotJoinOwnIdea:   422,
	CodeExportTooLarge:      422,
	CodeLoginThrottled:      429,
	CodeRateLimited:         429,
	CodeInternalError:       500,
	CodeStorageUnavailable:  503,
	CodeServiceUnavailable:  503,
	CodeRequired:            422,
	CodeInvalidFormat:       422,
	CodeOutOfRange:          422,
	CodeInvalidEnum:         422,
}
