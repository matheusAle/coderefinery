import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { Title } from "../src/Title";

test("renders the heading", () => {
  render(<Title text="Hello" />);
  expect(screen.getByRole("heading").textContent).toBe("Hello");
});
