const { price } = require("../lib/pricing");

test("price returns a number", () => {
  expect(price(10, 1.0)).not.toBeNull();
});
