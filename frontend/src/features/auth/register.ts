import { ApiError, apiFetch } from "@/shared/api";
import { checkEmail, checkName } from "@/shared/text";

export const REGISTER_FIELDS = [
  "empNo",
  "name",
  "email",
  "password",
  "confirmPassword",
] as const;
export type RegisterField = (typeof REGISTER_FIELDS)[number];

export interface RegisterValues {
  empNo: string;
  name: string;
  email: string;
  password: string;
  confirmPassword: string;
}

/** A field problem: `code` is an `errors.<code>` key, or `PASSWORD_MISMATCH` (client only). */
export type FieldProblems = Partial<Record<RegisterField, string>>;

const NAME_MAX = 50;

/**
 * Same checks, same order as the server for the format step (PRD §6.3, §6.2),
 * one problem per field. The password policy is the server's (and F1b-01's) job;
 * only "required" and "matches the confirmation" are checked here.
 */
export function precheck(v: RegisterValues): {
  values: Omit<RegisterValues, "confirmPassword">;
  problems: FieldProblems;
} {
  const problems: FieldProblems = {};
  if (v.empNo.trim() === "") problems.empNo = "REQUIRED";
  else if (!/^[0-9]{6}$/.test(v.empNo)) problems.empNo = "INVALID_FORMAT";

  const name = checkName(v.name, NAME_MAX);
  if (name.problem) problems.name = name.problem;

  const email = checkEmail(v.email);
  if (email.problem) problems.email = email.problem;

  if (v.password === "") problems.password = "REQUIRED";
  if (v.confirmPassword !== v.password)
    problems.confirmPassword = "PASSWORD_MISMATCH";

  return {
    values: {
      empNo: v.empNo,
      name: name.cleaned,
      email: email.cleaned,
      password: v.password,
    },
    problems,
  };
}

export interface RegisterResult {
  empNo: string;
  name: string;
  email: string;
}

/** POST /auth/register. Creates no session; the caller sends the user to /login. */
export async function register(body: {
  empNo: string;
  name: string;
  email: string;
  password: string;
}): Promise<RegisterResult> {
  const res = await apiFetch("/auth/register", { method: "POST", body });
  return (await res.json()) as RegisterResult;
}

const isField = (f: string): f is RegisterField =>
  (REGISTER_FIELDS as readonly string[]).includes(f);

/**
 * Maps a failed register call to field problems. A 409 names its field, a 422 lists
 * `errors[]`. `rest` is what has no field of its own (CSRF, 503, …) and is shown
 * as a form-level message.
 */
export function mapServerError(e: unknown): {
  problems: FieldProblems;
  rest: ApiError | null;
} {
  if (!(e instanceof ApiError))
    return { problems: {}, rest: new ApiError({ status: 0, code: "UNKNOWN" }) };
  if (e.code === "EMP_NO_TAKEN")
    return { problems: { empNo: "EMP_NO_TAKEN" }, rest: null };
  if (e.code === "EMAIL_TAKEN")
    return { problems: { email: "EMAIL_TAKEN" }, rest: null };
  const problems: FieldProblems = {};
  for (const fe of e.errors) {
    // the first error of a field wins; a password lists every violated rule, the top code covers them
    if (isField(fe.field) && problems[fe.field] === undefined)
      problems[fe.field] =
        fe.field === "password" && e.code === "PASSWORD_POLICY"
          ? "PASSWORD_POLICY"
          : fe.code;
  }
  return { problems, rest: Object.keys(problems).length > 0 ? null : e };
}
