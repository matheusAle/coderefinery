// A module path from <rootDir> (moduleDirectories), not a relative import.
const { grade } = require("lib/grade");

test("50 passes", () => {
  expect(grade(50)).toBe("pass");
});
