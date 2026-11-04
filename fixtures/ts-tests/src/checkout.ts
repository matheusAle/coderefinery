import { applyCoupon, price, tierRate } from "./pricing";

export function checkout(items: number[], points: number, coupon: number): number {
  let total = 0.0;
  for (const item of items) {
    total += item;
  }
  return applyCoupon(price(total, tierRate(points)), coupon);
}
