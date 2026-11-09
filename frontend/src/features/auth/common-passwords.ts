import { foldCase, type CommonCheck } from "./password-policy";

/** Builds the lookup from the list file: one entry per line, folded, `\r` and empty lines dropped. */
export function parseCommonList(text: string): CommonCheck {
  const set = new Set<string>();
  for (const line of text.split("\n")) {
    const l = line.endsWith("\r") ? line.slice(0, -1) : line;
    if (l !== "") set.add(foldCase(l));
  }
  // the whole password as typed: nothing trimmed, an entry inside a longer password does not match
  return (pw) => set.has(foldCase(pw));
}

let loading: Promise<CommonCheck> | undefined;

/**
 * Loads the bundled Top-10,000 list (the backend's own file, NOTICE next to it)
 * as a separate chunk, once. Only the register page calls this, so no other
 * page downloads it (Frontend.md §5.6). A failed load is not cached, so a
 * later mount may retry.
 */
export function loadCommonPasswords(): Promise<CommonCheck> {
  loading ??= import("passwords/common-passwords.txt?raw").then(
    (m) => parseCommonList(m.default),
    (err: unknown) => {
      loading = undefined;
      throw err;
    },
  );
  return loading;
}
