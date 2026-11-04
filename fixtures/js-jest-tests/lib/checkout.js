const { applyCoupon, price, tierRate } = require("./pricing");

function checkout(items, points, coupon) {
  let total = 0.0;
  for (const item of items) {
    total += item;
  }
  return applyCoupon(price(total, tierRate(points)), coupon);
}

module.exports = { checkout };
