import pytest

from shop.pricing import Cart, bucket_totals, eligible, parse_quantity, shipping


def test_cart_total_for_member():
    cart = Cart("ann")
    assert cart.add("book", 60, 2) == 1
    assert cart.subtotal() == 120
    assert cart.total(member=True) == 118.8


def test_cart_total_for_guest():
    cart = Cart("bob")
    cart.add("pen", 10)
    assert cart.total() == 11.0


def test_cart_rejects_bad_items():
    with pytest.raises(ValueError):
        Cart("x").add("bad", -1)


def test_shipping_us():
    assert shipping("US", 3) == 5
    assert shipping("FR", 1) == 22


def test_parse_quantity():
    assert parse_quantity("4") == 4
    assert parse_quantity("abc") == 0


def test_eligible():
    assert eligible({"active": True, "age": 30})
    assert not eligible({"active": False, "age": 30})


def test_bucket_totals():
    orders = [{"amount": 60}, {"amount": 5}, {"amount": 7, "void": True}]
    assert bucket_totals(orders) == {"big": 60, "small": 5}
