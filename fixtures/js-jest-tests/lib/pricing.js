function tierRate(points) {
  if (points >= 100) {
    return 0.8;
  }
  if (points >= 50) {
    return 0.9;
  }
  return 1.0;
}

function price(total, rate) {
  if (total <= 0) {
    return 0;
  }
  return total * rate;
}

function applyCoupon(total, coupon) {
  const remaining = total - coupon;
  if (remaining < 0) {
    return 0;
  }
  return remaining;
}

module.exports = { tierRate, price, applyCoupon };
