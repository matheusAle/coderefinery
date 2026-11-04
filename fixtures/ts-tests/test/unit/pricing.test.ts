import { describe, expect, it } from "vitest";
import { applyCoupon, price, tierRate } from "../../src/pricing";

describe("tierRate", () => {
  it.each([
    ["gold", 100, 0.8],
    ["silver", 50, 0.9],
  ])("rate for %s", (_name, points, rate) => {
    expect(tierRate(points)).toBe(rate);
  });
});

describe("price", () => {
  it("of nothing is zero", () => {
    expect(price(0, 1.0)).toBe(0);
  });
  it("applies the rate", () => {
    expect(price(200, 0.8)).toBe(160);
    expect(price(0.5, 1.0)).toBe(0.5);
  });
});

describe("applyCoupon", () => {
  it("subtracts", () => {
    expect(applyCoupon(10, 3)).toBe(7);
    expect(applyCoupon(10, 9.5)).toBe(0.5);
  });
  it("never goes negative", () => {
    expect(applyCoupon(3, 5)).toBe(0);
  });
});
