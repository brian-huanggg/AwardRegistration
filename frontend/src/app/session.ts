import { useQuery, type QueryClient } from "@tanstack/react-query";
import type { DataRouter } from "react-router";
import { ApiError, apiFetch } from "@/shared/api";
import { findRoute } from "./access";
import type { Me } from "@/features/auth/login";

export type { Me };

export const meKey = ["me"] as const;

/** A 401 means visitor → null (Frontend.md §6.3 rule 1). */
export async function fetchMe(signal?: AbortSignal): Promise<Me | null> {
  try {
    const res = await apiFetch("/auth/me", {
      signal,
      allowUnauthenticated: true,
    });
    return (await res.json()) as Me;
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
}

/**
 * Why `me` became null. The route gate redirects a visitor on a protected route to `/login?next=…`
 * in the same render that clears `me`, racing the explicit navigation of the code that cleared it.
 * Recording the reason lets the gate send the same destination, so the race has one outcome.
 */
let leaving: "signedOut" | "sessionEnded" | null = null;
export const leavingReason = () => leaving;

/**
 * A new identity (login) or none (logout): nothing cached for the previous one may survive.
 * `me` itself is kept as a query so the observers of the route gate see the new value.
 */
export function replaceIdentity(client: QueryClient, me: Me | null): void {
  leaving = me === null ? "signedOut" : null;
  client.removeQueries({ predicate: (q) => q.queryKey[0] !== meKey[0] });
  client.setQueryData(meKey, me);
}

/** Router state a 401 redirect leaves for the login page to show "your session ended" (Frontend.md §5.2). */
export interface SessionEndedState {
  notice: "sessionEnded";
}

/** Router state `/register` leaves for the login page to confirm the account (not in the URL, Frontend.md §6). */
export interface RegisteredState {
  notice: "registered";
}

/**
 * Global 401 (Frontend.md §5.2): forget the user and go to `/login?next=<current>`.
 * Already on a public route (login, register) there is nothing to leave, so only `me` is cleared.
 */
export function handleSessionEnded(
  client: QueryClient,
  router: DataRouter,
): void {
  leaving = "sessionEnded";
  client.setQueryData(meKey, null);
  const { pathname, search } = router.state.location;
  if (findRoute(pathname)?.access === "public") return;
  const state: SessionEndedState = { notice: "sessionEnded" };
  void router.navigate(`/login?next=${encodeURIComponent(pathname + search)}`, {
    replace: true,
    state,
  });
}

export function useMe() {
  return useQuery({
    queryKey: meKey,
    queryFn: ({ signal }) => fetchMe(signal),
    retry: false,
    staleTime: 60_000,
  });
}
