/**
 * `can(action, { me, idea, state })` — pure UI hint (Frontend.md §5.4). Hiding or disabling a
 * control is never a security measure: the server decides, and a 403/404 from a wrong guess is
 * handled like any other error.
 */
export type Action =
  "createIdea" | "editIdea" | "replaceSlides" | "deleteIdea" | "downloadSlides";

export interface Actor {
  empNo: string;
  role: "USER" | "ADMIN";
}

export interface IdeaRef {
  leader: { empNo: string };
  members: readonly { empNo: string }[];
}

export type SubmissionState =
  "NOT_OPEN" | "OPEN" | "CLOSED" | (string & {}) | undefined;

export interface Context {
  me: Actor;
  idea?: IdeaRef;
  state?: SubmissionState;
}

export interface Decision {
  /** The control is offered at all. */
  visible: boolean;
  /** Offered but not usable now (outside the submission window). */
  enabled: boolean;
}

const HIDDEN: Decision = { visible: false, enabled: false };

export function can(action: Action, { me, idea, state }: Context): Decision {
  const open = state === "OPEN";
  if (me.role === "ADMIN") {
    // PRD §7: Admin downloads, replaces slides and deletes, not limited by the deadline; nothing else.
    return action === "downloadSlides" ||
      action === "replaceSlides" ||
      action === "deleteIdea"
      ? { visible: true, enabled: true }
      : HIDDEN;
  }
  if (action === "createIdea") return { visible: true, enabled: open };
  if (!idea) return HIDDEN;
  const leader = idea.leader.empNo === me.empNo;
  const member = idea.members.some((m) => m.empNo === me.empNo);
  switch (action) {
    case "downloadSlides":
      return leader || member ? { visible: true, enabled: true } : HIDDEN;
    case "editIdea":
    case "replaceSlides":
    case "deleteIdea":
      return leader ? { visible: true, enabled: open } : HIDDEN;
  }
}
