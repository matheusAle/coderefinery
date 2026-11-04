"""Checkout: the pricing pipeline end to end."""

from billing.pricing import apply_coupon, price, tier_rate


def checkout(items, points, coupon):
    return apply_coupon(price(sum(items), tier_rate(points)), coupon)
