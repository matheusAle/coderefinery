export function tierRate(points: number): number {
  if (points >= 100) {
    return 0.8;
  }
  if (points >= 50) {
    return 0.9;
  }
  return 1.0;
}

export function price(total: number, rate: number): number {
  if (total <= 0) {
    return 0;
  }
  return total * rate;
}

export function applyCoupon(total: number, coupon: number): number {
  const remaining = total - coupon;
  if (remaining < 0) {
    return 0;
  }
  return remaining;
}
