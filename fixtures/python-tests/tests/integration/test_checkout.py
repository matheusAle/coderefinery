from billing.checkout import checkout


def test_checkout_gold_member():
    assert checkout([100, 100], 100, 0) == 160
