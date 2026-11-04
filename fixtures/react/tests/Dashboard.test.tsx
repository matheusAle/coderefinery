import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { Dashboard } from "../src/Dashboard";

afterEach(cleanup);

test("logs out from the sidebar", () => {
  const onLogout = vi.fn();
  const user = { name: "Ada", avatarUrl: "ada.png" };
  render(<Dashboard user={user} theme="dark" locale="en" notifications={3} onLogout={onLogout} />);
  expect(screen.getByText("3")).toBeTruthy();
  fireEvent.click(screen.getByText("Log out"));
  expect(onLogout).toHaveBeenCalledTimes(1);
});
