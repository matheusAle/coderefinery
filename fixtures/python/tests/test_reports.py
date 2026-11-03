from shop.reports import Ledger, summarize_orders


def test_summarize_orders():
    lines = summarize_orders([{"id": "a", "amount": 2}, {"id": "b", "amount": 0}])
    assert lines == ["a: 2.00", "total: 2.00"]


def test_ledger_balance():
    ledger = Ledger()
    ledger.record(5)
    ledger.record(3, "void")
    assert ledger.balance() == 5
