import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useCountdown } from "./useCountdown";

describe("useCountdown", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("counts down in whole seconds and stops at zero", () => {
    const { result } = renderHook(() => useCountdown());
    expect(result.current.remaining).toBe(0);
    act(() => result.current.start(3));
    expect(result.current.remaining).toBe(3);
    act(() => void vi.advanceTimersByTime(1000));
    expect(result.current.remaining).toBe(2);
    act(() => void vi.advanceTimersByTime(2000));
    expect(result.current.remaining).toBe(0);
  });

  it("treats 0 as at least one second", () => {
    const { result } = renderHook(() => useCountdown());
    act(() => result.current.start(0));
    expect(result.current.remaining).toBe(1);
  });

  it("restarts from a new value", () => {
    const { result } = renderHook(() => useCountdown());
    act(() => result.current.start(10));
    act(() => void vi.advanceTimersByTime(4000));
    act(() => result.current.start(30));
    expect(result.current.remaining).toBe(30);
  });
});
