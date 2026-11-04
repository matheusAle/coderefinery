import pytest

from billing.pricing import apply_coupon, price, tier_rate


@pytest.mark.parametrize("points,rate", [(100, 0.8), (50, 0.9)], ids=["gold", "silver"])
def test_tier_rate(points, rate):
    assert tier_rate(points) == rate


def test_price_of_nothing_is_zero():
    assert price(0, 1.0) == 0


def test_price_applies_the_rate():
    assert price(200, 0.8) == 160
    assert price(0.5, 1.0) == 0.5


def test_coupon_subtracts():
    assert apply_coupon(10, 3) == 7
    assert apply_coupon(10, 9.5) == 0.5


def test_coupon_never_goes_negative():
    assert apply_coupon(3, 5) == 0
