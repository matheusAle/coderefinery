import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, test } from "vitest";
import { Badge } from "../src/Badge";

afterEach(cleanup);

test("shows the count", () => {
  render(<Badge label="Inbox" count={5} />);
  expect(screen.getByText("Inbox 5")).toBeTruthy();
});

test("caps the count at 99+", () => {
  render(<Badge label="Inbox" count={120} />);
  expect(screen.getByText("Inbox 99+")).toBeTruthy();
});
