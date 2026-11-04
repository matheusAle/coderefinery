import { act, renderHook } from "@testing-library/react";
import { expect, test } from "vitest";
import { useCounter } from "../src/useCounter";

test("counts up to max and records it", () => {
  const { result } = renderHook(() => useCounter(8, 1, 9));
  expect(result.current.parity).toBe("even");
  act(() => result.current.increment());
  act(() => result.current.increment());
  expect(result.current.count).toBe(9);
  expect(result.current.history).toEqual([9]);
  act(() => result.current.reset());
  expect(result.current.count).toBe(8);
});
