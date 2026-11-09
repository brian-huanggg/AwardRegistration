import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { axe } from "vitest-axe";
import { apiFetch, markActivity } from "@/shared/api";
import { server } from "@/shared/api/mocks/server";
import { initI18n } from "@/shared/i18n";
import { IdleWarning } from "./IdleWarning";

const API = "http://localhost:3000/api/v1";
const MIN = 60_000;
const DIALOG = { name: "Are you still there?" };

beforeEach(async () => {
  await initI18n("en");
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "Date"], shouldAdvanceTime: true });
  markActivity();
});
afterEach(() => vi.useRealTimers());

const advance = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));

describe("IdleWarning", () => {
  it("opens at 28 minutes and ends the session at 30", async () => {
    const onExpire = vi.fn();
    render(<IdleWarning signedIn onExpire={onExpire} />);
    await advance(28 * MIN - 1);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    await advance(1);
    expect(screen.getByRole("alertdialog", DIALOG)).toBeInTheDocument();
    expect(onExpire).not.toHaveBeenCalled();
    await advance(2 * MIN);
    expect(onExpire).toHaveBeenCalledTimes(1);
  });

  it("Continue calls GET /auth/me, closes the dialog and restarts the clock", async () => {
    let calls = 0;
    server.use(
      http.get(`${API}/auth/me`, () => {
        calls += 1;
        return HttpResponse.json({ empNo: "089290" });
      }),
    );
    const onExpire = vi.fn();
    render(<IdleWarning signedIn onExpire={onExpire} />);
    await advance(28 * MIN);
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTimeAsync });
    await user.click(screen.getByRole("button", { name: "Continue" }));
    await vi.waitFor(() => expect(calls).toBe(1));
    await vi.waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    await advance(29 * MIN);
    expect(onExpire).not.toHaveBeenCalled();
    await advance(MIN);
    expect(onExpire).toHaveBeenCalledTimes(1);
  });

  it("any answered request restarts the clock before the warning", async () => {
    server.use(http.get(`${API}/deadline`, () => HttpResponse.json({})));
    render(<IdleWarning signedIn onExpire={vi.fn()} />);
    await advance(20 * MIN);
    await apiFetch("/deadline");
    await advance(20 * MIN);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    await advance(8 * MIN);
    expect(screen.getByRole("alertdialog", DIALOG)).toBeInTheDocument();
  });

  it("does nothing for a visitor", async () => {
    const onExpire = vi.fn();
    render(<IdleWarning signedIn={false} onExpire={onExpire} />);
    await advance(40 * MIN);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(onExpire).not.toHaveBeenCalled();
  });

  it("has no axe violations", async () => {
    render(<IdleWarning signedIn onExpire={vi.fn()} />);
    await advance(28 * MIN);
    vi.useRealTimers();
    expect((await axe(document.body)).violations).toEqual([]);
  });
});
