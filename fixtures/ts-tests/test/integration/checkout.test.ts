import { expect, it } from "vitest";
import { checkout } from "../../src/checkout";

it("checks out a gold member", () => {
  expect(checkout([100, 100], 100, 0)).toBe(160);
});
