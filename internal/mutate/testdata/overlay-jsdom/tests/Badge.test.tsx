import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { Badge } from "../src/Badge";

test("adds the counts", () => {
  render(<Badge label="Inbox" counts={[2, 3]} />);
  expect(screen.getByText("Inbox 5")).toBeTruthy();
});

test("caps the count at 99+", () => {
  render(<Badge label="Inbox" counts={[100, 20]} />);
  expect(screen.getByText("Inbox 99+")).toBeTruthy();
});
