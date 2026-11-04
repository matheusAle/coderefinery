from billing.pricing import price, receipt, shipping


def test_price():
    assert price([(10, 2)], "gold") == 16.0
    assert price([(10, 1)]) == 10.0


def test_receipt_of_nothing():
    assert receipt([]) == 0


def test_shipping():
    assert shipping("US") == 5
    assert shipping("FR") == 20
