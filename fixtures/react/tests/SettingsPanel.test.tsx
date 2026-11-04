import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { SettingsPanel } from "../src/SettingsPanel";

afterEach(cleanup);

test("saves", () => {
  const onSave = vi.fn();
  render(<SettingsPanel title="Settings" darkMode compact={false} onSave={onSave} />);
  fireEvent.click(screen.getByText("Save"));
  expect(screen.getByText("Settings").tagName).toBe("H2");
  expect(onSave).toHaveBeenCalledTimes(1);
});
