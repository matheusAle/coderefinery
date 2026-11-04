from billing.pricing import price


def test_price_returns_a_number():
    assert price(10, 1.0) is not None
