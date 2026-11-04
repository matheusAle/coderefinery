import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { Cart } from "../src/Cart";

afterEach(cleanup);

test("an empty cart says so", () => {
  render(<Cart items={[]} currency="eur" onCheckout={vi.fn()} />);
  expect(screen.getByText("Your cart is empty")).toBeTruthy();
});

test("checks out the discounted total", () => {
  const onCheckout = vi.fn();
  const items = [{ id: "a", price: 30, quantity: 2 }];
  render(<Cart items={items} currency="eur" couponCode="HALF" onCheckout={onCheckout} />);
  expect(screen.getByText("Total: 30 EUR")).toBeTruthy();
  fireEvent.click(screen.getByText("Checkout"));
  expect(onCheckout).toHaveBeenCalledWith(30);
});
