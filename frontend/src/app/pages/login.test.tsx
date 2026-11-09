import { QueryClient } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter } from "react-router";
import { axe } from "vitest-axe";
import { clearCsrfToken } from "@/shared/api";
import { problem } from "@/shared/api/mocks/problem";
import { server } from "@/shared/api/mocks/server";
import { initI18n } from "@/shared/i18n";
import { App } from "../App";
import { routes } from "../router";

const API = "http://localhost:3000/api/v1";
const USER = { empNo: "089290", name: "Wang", email: "w@x.io", role: "USER" };
type Account = typeof USER | { empNo: string; name: string; email: null; role: "ADMIN" };

/** A visitor on `path`; `account` is who a successful login returns. `GET /auth/me` follows the session. */
async function open(path: string, account: Account = USER) {
  clearCsrfToken();
  let session: Account | null = null;
  const calls: { login: unknown[]; logoutCsrf: (string | null)[] } = {
    login: [],
    logoutCsrf: [],
  };
  server.use(
    http.get(`${API}/auth/me`, () =>
      session ? HttpResponse.json(session) : problem(401, "UNAUTHENTICATED"),
    ),
    http.get(`${API}/deadline`, () =>
      HttpResponse.json({
        deadline: null,
        state: "OPEN",
        serverTime: "2026-11-08T00:00:00.000Z",
      }),
    ),
    http.get(`${API}/auth/csrf`, () =>
      HttpResponse.json({ token: session ? "tok-session" : "tok-anon" }),
    ),
    http.post(`${API}/auth/login`, async ({ request }) => {
      calls.login.push(await request.json());
      session = account;
      return HttpResponse.json(account);
    }),
    http.post(`${API}/auth/logout`, ({ request }) => {
      calls.logoutCsrf.push(request.headers.get("X-CSRF-Token"));
      session = null;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  await initI18n("en");
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const view = render(<App router={router} client={new QueryClient()} />);
  await screen.findByRole("heading", {
    level: 1,
    name: "Sign in to your account",
  });
  return { router, calls, user: userEvent.setup(), ...view };
}

async function signIn(
  user: ReturnType<typeof userEvent.setup>,
  empNo = "089290",
  password = "secret",
) {
  await user.type(screen.getByLabelText("Employee ID"), empNo);
  await user.type(screen.getByLabelText("Password"), password);
  await user.click(screen.getByRole("button", { name: "Sign in" }));
}

describe("Login page", () => {
  it("has labelled, autocompleted fields and no axe violations", async () => {
    const { container } = await open("/login");
    expect(screen.getByLabelText("Employee ID")).toHaveAttribute(
      "autocomplete",
      "username",
    );
    expect(screen.getByLabelText("Password")).toHaveAttribute(
      "autocomplete",
      "current-password",
    );
    expect((await axe(container)).violations).toEqual([]);
  });

  it("signs a User in and lands on /ideas", async () => {
    const { router, calls, user } = await open("/login");
    await signIn(user, " 089290 ", "secret");
    await waitFor(() => expect(router.state.location.pathname).toBe("/ideas"));
    expect(calls.login).toEqual([{ empNo: "089290", password: "secret" }]);
  });

  it("follows a role-allowed next", async () => {
    const { router, user } = await open("/login?next=%2Fjoin-requests");
    await signIn(user);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/join-requests"),
    );
  });

  it.each([
    "//evil.example",
    "https://evil.example/x",
    "/admin/deadline",
    "/nope",
    "\\\\evil.example",
  ])("ignores next=%s", async (next) => {
    const { router, user } = await open(
      `/login?next=${encodeURIComponent(next)}`,
    );
    await signIn(user);
    await waitFor(() => expect(router.state.location.pathname).toBe("/ideas"));
    expect(router.state.location.search).toBe("");
  });

  it("shows one message for a wrong password, clears the password and focuses the message", async () => {
    const { user, container } = await open("/login");
    server.use(
      http.post(`${API}/auth/login`, () => problem(401, "INVALID_CREDENTIALS")),
    );
    await signIn(user, "089290", "wrong");
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      "The employee ID or password is incorrect.",
    );
    expect(alert).toHaveFocus();
    expect(screen.getByLabelText("Password")).toHaveValue("");
    expect(screen.getByLabelText("Employee ID")).toHaveValue("089290");
    expect((await axe(container)).violations).toEqual([]);
  });

  it("shows the throttle message", async () => {
    const { user } = await open("/login");
    server.use(
      http.post(`${API}/auth/login`, () =>
        problem(429, "LOGIN_THROTTLED", {}, { "Retry-After": "60" }),
      ),
    );
    await signIn(user);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Too many sign-in attempts",
    );
  });

  it("disables sign-in for Retry-After seconds, then re-enables it", async () => {
    // only the interval and the monotonic clock are faked; userEvent and msw keep real timers
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
    let now = 1_000;
    vi.spyOn(performance, "now").mockImplementation(() => now);
    const advance = (ms: number) =>
      act(() => {
        now += ms;
        vi.advanceTimersByTime(ms);
      });
    try {
      const { user } = await open("/login");
      server.use(
        http.post(`${API}/auth/login`, () =>
          problem(429, "LOGIN_THROTTLED", {}, { "Retry-After": "3" }),
        ),
      );
      await signIn(user);
      expect(
        await screen.findByRole("button", { name: "Try again in 3 s" }),
      ).toBeDisabled();
      advance(1000);
      expect(
        screen.getByRole("button", { name: "Try again in 2 s" }),
      ).toBeDisabled();
      advance(2000);
      expect(screen.getByRole("button", { name: "Sign in" })).toBeEnabled();
      expect(screen.getByRole("status")).toHaveTextContent(
        "You can sign in again.",
      );
    } finally {
      vi.restoreAllMocks();
      vi.useRealTimers();
    }
  });

  it("shows the registered and session-ended notices", async () => {
    const router = createMemoryRouter(routes, {
      initialEntries: [
        { pathname: "/login", state: { notice: "sessionEnded" } },
      ],
    });
    server.use(
      http.get(`${API}/auth/me`, () => problem(401, "UNAUTHENTICATED")),
      http.get(`${API}/deadline`, () =>
        HttpResponse.json({
          deadline: null,
          state: "OPEN",
          serverTime: "2026-11-08T00:00:00.000Z",
        }),
      ),
    );
    await initI18n("en");
    render(<App router={router} client={new QueryClient()} />);
    expect(await screen.findByRole("status", { name: "" })).toBeInTheDocument();
    expect(
      await screen.findByText("Your session has ended. Please sign in again."),
    ).toBeInTheDocument();
  });
});
