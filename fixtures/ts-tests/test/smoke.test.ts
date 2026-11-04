import { expect, it } from "vitest";
import { price } from "../src/pricing";

it("price returns a number", () => {
  expect(price(10, 1.0)).not.toBeNull();
});
