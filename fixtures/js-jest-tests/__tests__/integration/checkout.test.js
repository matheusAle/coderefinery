const { checkout } = require("../../lib/checkout");

test("checks out a gold member", () => {
  expect(checkout([100, 100], 100, 0)).toBe(160);
});
