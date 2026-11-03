import { describe, expect, it } from "vitest";
import { Inventory, discount, label, shippingZone } from "../src/inventory";

describe("Inventory", () => {
  it("adds and values items", () => {
    const inv = new Inventory();
    expect(inv.add({ sku: "a", price: 2, qty: 3 })).toBe(1);
    expect(inv.add({ sku: "b", price: 5 })).toBe(2);
    expect(inv.value()).toBe(11);
    expect(inv.quantity("a")).toBe(3);
    expect(inv.quantity("zzz")).toBe(0);
  });

  it("rejects invalid items", () => {
    expect(() => new Inventory().add({ sku: "", price: 1 })).toThrow("invalid item");
  });
});

describe("helpers", () => {
  it("labels with tags", () => {
    expect(label({ sku: "a", price: 1, tags: ["x", "y"] })).toBe("a [x,y]");
  });

  it("discounts members", () => {
    expect(discount(100, true)).toBe(90);
  });

  it("zones", () => {
    expect(shippingZone("US")).toBe(1);
    expect(shippingZone("BR")).toBe(3);
  });
});
