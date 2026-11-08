import { ERROR_CODES, type ErrorCode } from './error-codes.generated';

export type { ErrorCode };

export interface FieldError {
  field: string;
  code: string;
  [extra: string]: unknown;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: ErrorCode | 'UNKNOWN';
  readonly errors: FieldError[];
  readonly requestId: string | undefined;
  readonly retryAfterSeconds: number | undefined;

  constructor(init: {
    status: number;
    code: ErrorCode | 'UNKNOWN';
    detail?: string;
    errors?: FieldError[];
    requestId?: string;
    retryAfterSeconds?: number;
  }) {
    super(init.detail ?? init.code);
    this.name = 'ApiError';
    this.status = init.status;
    this.code = init.code;
    this.errors = init.errors ?? [];
    this.requestId = init.requestId;
    this.retryAfterSeconds = init.retryAfterSeconds;
  }
}

const known = new Set<string>(ERROR_CODES.map((e) => e.code));

// Non-JSON error bodies (e.g. an ingress 413) map from the HTTP status (Frontend.md §5.2).
const statusFallback: Partial<Record<number, ErrorCode>> = {
  400: 'MALFORMED_REQUEST',
  401: 'UNAUTHENTICATED',
  404: 'NOT_FOUND',
  405: 'METHOD_NOT_ALLOWED',
  406: 'NOT_ACCEPTABLE',
  413: 'FILE_TOO_LARGE',
  500: 'INTERNAL_ERROR',
  503: 'SERVICE_UNAVAILABLE',
};

function parseRetryAfter(value: string | null): number | undefined {
  if (value === null || !/^\d+$/.test(value.trim())) return undefined;
  return Number(value.trim());
}

function asFieldErrors(v: unknown): FieldError[] {
  if (!Array.isArray(v)) return [];
  return v.filter(
    (e): e is FieldError =>
      typeof e === 'object' && e !== null && typeof (e as FieldError).field === 'string' && typeof (e as FieldError).code === 'string',
  );
}

/** Builds an ApiError from a non-2xx response. Never throws. */
export async function parseApiError(res: Response): Promise<ApiError> {
  const retryAfterSeconds = parseRetryAfter(res.headers.get('Retry-After'));
  const requestHeader = res.headers.get('X-Request-Id') ?? undefined;
  const isProblem = (res.headers.get('Content-Type') ?? '').toLowerCase().startsWith('application/problem+json');

  if (isProblem) {
    try {
      const body: unknown = await res.json();
      if (typeof body === 'object' && body !== null) {
        const b = body as Record<string, unknown>;
        const code = typeof b.code === 'string' && known.has(b.code) ? (b.code as ErrorCode) : 'UNKNOWN';
        return new ApiError({
          status: res.status,
          code,
          detail: typeof b.detail === 'string' ? b.detail : undefined,
          errors: asFieldErrors(b.errors),
          requestId: typeof b.requestId === 'string' ? b.requestId : requestHeader,
          retryAfterSeconds,
        });
      }
    } catch {
      // fall through to the status mapping
    }
  }
  return new ApiError({
    status: res.status,
    code: statusFallback[res.status] ?? 'UNKNOWN',
    requestId: requestHeader,
    retryAfterSeconds,
  });
}
