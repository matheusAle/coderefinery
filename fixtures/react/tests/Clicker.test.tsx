import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test } from "vitest";
import { Clicker } from "../src/Clicker";

afterEach(cleanup);

test("counts clicks; the toggle keeps its own state", () => {
  render(<Clicker />);
  for (let i = 0; i < 3; i++) fireEvent.click(screen.getByText("add"));
  fireEvent.click(screen.getByText("off"));
  expect(screen.getByRole("status").textContent).toBe("3");
  expect(screen.getByText("on")).toBeTruthy();
});
