import { Check, Circle, X } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/shared/ui/utils";
import { loadCommonPasswords } from "./common-passwords";
import {
  checkPassword,
  empNoForPolicy,
  PASSWORD_RULES,
  type CommonCheck,
  type PasswordRule,
} from "./password-policy";

/** `idle`: nothing typed yet; `server`: the blocklist is not (yet) loaded, the server checks this rule. */
type Status = "met" | "unmet" | "idle" | "server";

type ListState = { check: CommonCheck } | { failed: true } | null;

/** Loads the blocklist chunk on mount; until it arrives (or if it fails) COMMON_PASSWORD is left to the server. */
function useCommonList(): CommonCheck | undefined {
  const [state, setState] = useState<ListState>(null);
  useEffect(() => {
    let live = true;
    loadCommonPasswords().then(
      (check) => live && setState({ check }),
      () => live && setState({ failed: true }),
    );
    return () => {
      live = false;
    };
  }, []);
  return state && "check" in state ? state.check : undefined;
}

const ICON = { met: Check, unmet: X, idle: Circle, server: Circle } as const;

/**
 * Live checklist of the 8 password rules (PRD §6.4, Frontend.md §5.6), keyed by
 * the rule ids of PASSWORD_POLICY. It is not a live region: the password field
 * points at it with `aria-describedby`, so the current state is read on focus
 * instead of after every keystroke. State is carried by text, not colour alone.
 */
export function PasswordChecklist({
  id,
  password,
  empNo,
}: {
  id: string;
  password: string;
  empNo: string;
}) {
  const { t } = useTranslation();
  const isCommon = useCommonList();
  const violated = new Set<PasswordRule>(
    checkPassword(password, empNoForPolicy(empNo), isCommon),
  );
  const status = (r: PasswordRule): Status => {
    if (password === "") return "idle";
    if (r === "COMMON_PASSWORD" && !isCommon) return "server";
    return violated.has(r) ? "unmet" : "met";
  };

  return (
    <div id={id} className="text-sm">
      <p className="text-muted-foreground">{t("passwordRules.title")}</p>
      <ul className="mt-1 space-y-0.5">
        {PASSWORD_RULES.map((r) => {
          const s = status(r);
          const Icon = ICON[s];
          return (
            <li
              key={r}
              data-rule={r}
              data-status={s}
              className={cn(
                "flex items-start gap-1.5",
                s === "met" && "text-emerald-700 dark:text-emerald-400",
                s === "unmet" && "text-destructive",
                (s === "idle" || s === "server") && "text-muted-foreground",
              )}
            >
              <Icon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
              <span>
                {t(`passwordRules.${r}`)}
                <span className="sr-only">
                  {" "}
                  ({t(`passwordRules.status.${s}`)})
                </span>
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
