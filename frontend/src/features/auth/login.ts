import { apiFetch } from "@/shared/api";

/** GET /auth/me and POST /auth/login body (docs/API/response-bodies.md); `email` is null for Admins. */
export interface Me {
  empNo: string;
  name: string;
  email: string | null;
  role: "USER" | "ADMIN";
}

/** POST /auth/login. Sets the session cookie; the body is the same shape as `GET /auth/me`. */
export async function login(body: {
  empNo: string;
  password: string;
}): Promise<Me> {
  const res = await apiFetch("/auth/login", { method: "POST", body });
  return (await res.json()) as Me;
}

/** POST /auth/logout: idempotent 204; `apiFetch` drops the session-bound CSRF token afterwards. */
export async function logout(): Promise<void> {
  await apiFetch("/auth/logout", { method: "POST" });
}
