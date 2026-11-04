from calc import total


def test_total_of_two_items():
    items = [(10, 2), (5, 1)]
    expected = 25
    assert total(items) == expected
    assert total(items[:1]) == 20


def test_total_skips_empty_quantities():
    items = [(10, 0), (5, 3)]
    expected = 15
    assert total(items) == expected
    assert total(items[:1]) == 0
