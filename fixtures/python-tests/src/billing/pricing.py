"""Prices with loyalty tiers and coupons."""


def tier_rate(points):
    if points >= 100:
        return 0.8
    if points >= 50:
        return 0.9
    return 1.0


def price(total, rate):
    if total <= 0:
        return 0
    return round(total * rate, 2)


def apply_coupon(total, coupon):
    remaining = total - coupon
    if remaining < 0:
        return 0
    return remaining
