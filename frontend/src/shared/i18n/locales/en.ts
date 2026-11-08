import type { ErrorCode } from '@/shared/api';

/** `UNKNOWN` covers a code the client does not know (parseApiError). */
export type ErrorKey = ErrorCode | 'UNKNOWN';

export const en = {
  app: { name: 'ESG Idea Contest' },
  common: {
    language: 'Language',
    save: 'Save',
    cancel: 'Cancel',
    retry: 'Try again',
    requestId: 'Request ID: {{id}}',
  },
  errors: {
    MALFORMED_REQUEST: 'The request could not be understood. Please reload the page and try again.',
    UNAUTHENTICATED: 'Your session has expired. Please sign in again.',
    INVALID_CREDENTIALS: 'The employee ID or password is incorrect.',
    CSRF_INVALID: 'Your session could not be verified. Please try again.',
    FORBIDDEN: 'You do not have permission to do this.',
    SUBMISSION_NOT_OPEN: 'Submissions have not opened yet.',
    SUBMISSION_CLOSED: 'Submissions are closed.',
    NOT_FOUND: 'The page or item you asked for was not found.',
    METHOD_NOT_ALLOWED: 'This action is not supported.',
    NOT_ACCEPTABLE: 'This action is not supported.',
    EMP_NO_TAKEN: 'This employee ID is already registered.',
    EMAIL_TAKEN: 'This email address is already registered.',
    IDEA_DUPLICATE: 'You already have an Idea with this name.',
    ALREADY_MEMBER: 'You are already a member of this Idea.',
    JOIN_REQUEST_EXISTS: 'You already have a join request for this Idea.',
    TEAM_FULL: 'This team is full.',
    JOIN_REQUEST_NOT_PENDING: 'This join request has already been handled.',
    VERSION_CONFLICT: 'This item was changed by someone else. Reload and try again.',
    FILE_TOO_LARGE: 'The file is too large (limit 5 MiB).',
    REQUEST_TOO_LARGE: 'The request is too large.',
    UNSUPPORTED_FILE_TYPE: 'This file type is not supported.',
    UNSUPPORTED_MEDIA_TYPE: 'This request format is not supported.',
    VALIDATION_FAILED: 'Please check the highlighted fields.',
    PASSWORD_POLICY: 'The password does not meet the requirements.',
    INVALID_NAME_CHARS: 'The name contains characters that are not allowed.',
    FILENAME_TOO_LONG: 'The file name is too long.',
    FILENAME_MISMATCH: 'The file name does not match the expected name.',
    EMPTY_FILE: 'The file is empty.',
    CANNOT_JOIN_OWN_IDEA: 'You cannot request to join your own Idea.',
    EXPORT_TOO_LARGE: 'Too many rows to export. Narrow the filters and try again.',
    LOGIN_THROTTLED: 'Too many sign-in attempts. Please try again later.',
    RATE_LIMITED: 'Too many requests. Please try again later.',
    INTERNAL_ERROR: 'Something went wrong on our side. Please try again.',
    STORAGE_UNAVAILABLE: 'File storage is temporarily unavailable. Please try again later.',
    SERVICE_UNAVAILABLE: 'The service is temporarily unavailable. Please try again later.',
    REQUIRED: 'This field is required.',
    INVALID_FORMAT: 'The format is not valid.',
    OUT_OF_RANGE: 'The value is out of range.',
    INVALID_ENUM: 'Please choose one of the listed options.',
    UNKNOWN: 'An unexpected error occurred. Please try again.',
  } satisfies Record<ErrorKey, string>,
};

export type Dictionary = typeof en;
