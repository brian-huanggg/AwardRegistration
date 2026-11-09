import { render, screen, waitFor } from "@testing-library/react";
import { axe } from "vitest-axe";
import { initI18n } from "@/shared/i18n";
import { PasswordChecklist } from "./PasswordChecklist";

const statuses = () =>
  Object.fromEntries(
    Array.from(document.querySelectorAll("li[data-rule]")).map((li) => [
      li.getAttribute("data-rule"),
      li.getAttribute("data-status"),
    ]),
  );

/** Renders and waits until the blocklist chunk is in (COMMON_PASSWORD no longer "server"). */
async function show(password: string, empNo = "") {
  const view = render(<PasswordChecklist id="rules" password={password} empNo={empNo} />);
  if (password !== "")
    await waitFor(() => expect(statuses().COMMON_PASSWORD).not.toBe("server"));
  return view;
}

beforeEach(async () => {
  await initI18n("en");
});

describe("PasswordChecklist", () => {
  it("lists the 8 rules in PRD §6.4 order, neutral while empty", async () => {
    await show("");
    expect(Object.keys(statuses())).toEqual([
      "LENGTH",
      "CHARSET",
      "COMPLEXITY",
      "COMMON_PASSWORD",
      "KEYBOARD_RUN",
      "SEQUENCE",
      "REPEAT",
      "CONTAINS_EMP_NO",
    ]);
    expect(new Set(Object.values(statuses()))).toEqual(new Set(["idle"]));
  });

  it("marks each rule met or not met as the password changes", async () => {
    const { rerender } = await show("password");
    expect(statuses()).toMatchObject({
      LENGTH: "unmet",
      CHARSET: "met",
      COMPLEXITY: "unmet",
      COMMON_PASSWORD: "unmet",
      KEYBOARD_RUN: "met",
    });
    rerender(<PasswordChecklist id="rules" password="Correct-Horse-1" empNo="089290" />);
    expect(new Set(Object.values(statuses()))).toEqual(new Set(["met"]));
  });

  it("checks the employee ID only when it is well-formed (as the server)", async () => {
    const { rerender } = await show("Correct-089290-Ab", "089290");
    expect(statuses().CONTAINS_EMP_NO).toBe("unmet");
    rerender(<PasswordChecklist id="rules" password="Correct-089290-Ab" empNo="08929" />);
    expect(statuses().CONTAINS_EMP_NO).toBe("met");
  });

  it("says the state in text, not colour alone (WCAG 1.4.1), and has no axe violations", async () => {
    const { container } = await show("password");
    expect(screen.getByText(/be 12–64 characters long/).textContent).toContain("(not met)");
    expect(screen.getByText(/use only printable ASCII/).textContent).toContain("(met)");
    expect((await axe(container)).violations).toEqual([]);
  });

  it("is not a live region (no announcement on every keystroke)", async () => {
    const { container } = await show("x");
    expect(container.querySelector("[aria-live],[role=status],[role=alert]")).toBeNull();
  });
});
