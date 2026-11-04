import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { TodoList } from "../src/TodoList";

afterEach(cleanup);

const todos = [
  { id: 1, text: "write", done: true },
  { id: 2, text: "test", done: false },
];

test("toggles a row and filters done todos", () => {
  const onToggle = vi.fn();
  render(<TodoList todos={todos} onToggle={onToggle} />);
  fireEvent.click(screen.getByText("test"));
  expect(onToggle).toHaveBeenCalledWith(2);
  fireEvent.change(screen.getByRole("combobox"), { target: { value: "done" } });
  expect(screen.queryByText("test")).toBeNull();
  expect(screen.getByText("write")).toBeTruthy();
});
