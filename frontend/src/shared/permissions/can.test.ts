import { can, type Action, type Actor } from "./can";

const ADMIN: Actor = { empNo: "000001", role: "ADMIN" };
const LEADER: Actor = { empNo: "089290", role: "USER" };
const MEMBER: Actor = { empNo: "089291", role: "USER" };
const OTHER: Actor = { empNo: "089299", role: "USER" };
const idea = { leader: { empNo: "089290" }, members: [{ empNo: "089291" }] };

const cell = (a: Action, me: Actor, state: "OPEN" | "CLOSED") => {
  const d = can(a, { me, idea, state });
  return d.visible ? (d.enabled ? "on" : "disabled") : "hidden";
};

describe("can()", () => {
  it.each([
    // action, admin, leader, member, other  (OPEN)
    ["createIdea", "hidden", "on", "on", "on"],
    ["editIdea", "hidden", "on", "hidden", "hidden"],
    ["replaceSlides", "on", "on", "hidden", "hidden"],
    ["deleteIdea", "on", "on", "hidden", "hidden"],
    ["downloadSlides", "on", "on", "on", "hidden"],
    ["approveMember", "hidden", "on", "hidden", "hidden"],
    ["removeMember", "hidden", "on", "hidden", "hidden"],
    ["leaveTeam", "hidden", "hidden", "on", "hidden"],
  ] as const)("%s while OPEN", (a, admin, leader, member, other) => {
    expect(
      [ADMIN, LEADER, MEMBER, OTHER].map((m) => cell(a, m, "OPEN")),
    ).toEqual([admin, leader, member, other]);
  });

  it("closes Leader writes but never the Admin ones", () => {
    expect(cell("replaceSlides", LEADER, "CLOSED")).toBe("disabled");
    expect(cell("deleteIdea", LEADER, "CLOSED")).toBe("disabled");
    expect(cell("approveMember", LEADER, "CLOSED")).toBe("disabled");
    expect(cell("removeMember", LEADER, "CLOSED")).toBe("disabled");
    expect(cell("leaveTeam", MEMBER, "CLOSED")).toBe("disabled");
    expect(cell("replaceSlides", ADMIN, "CLOSED")).toBe("on");
    expect(cell("deleteIdea", ADMIN, "CLOSED")).toBe("on");
    expect(cell("downloadSlides", LEADER, "CLOSED")).toBe("on");
  });

  it("disables writes while the state is unknown", () => {
    expect(can("deleteIdea", { me: LEADER, idea }).enabled).toBe(false);
  });
});
