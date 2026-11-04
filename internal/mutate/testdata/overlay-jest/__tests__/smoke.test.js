const { grade } = require("../lib/grade");

test("grade returns a string", () => {
  expect(typeof grade(10)).toBe("string");
});
