"""Reports with two near-duplicate summaries (dryer) and an untested branch-heavy function."""


def summarize_orders(orders):
    lines = []
    total = 0
    for order in orders:
        if order["amount"] > 0:
            total += order["amount"]
            lines.append(format_line(order["id"], order["amount"]))
    lines.append(format_line("total", total))
    return lines


def summarize_refunds(refunds):
    rows = []
    sum_ = 0
    for refund in refunds:
        if refund["value"] > 0:
            sum_ += refund["value"]
            rows.append(format_line(refund["ref"], refund["value"]))
    rows.append(format_line("total", sum_))
    return rows


def format_line(label, amount):
    return f"{label}: {amount:.2f}"


def _grade(score):
    if score >= 90:
        return "A"
    elif score >= 80:
        return "B"
    elif score >= 70:
        return "C"
    while score < 0:
        score += 10
    return "F" if score < 50 else "D"


class Ledger:
    def __init__(self):
        self.entries = []

    def record(self, amount, note=None):
        self.entries.append({"amount": amount, "note": note or "-"})

    def balance(self):
        return sum(e["amount"] for e in self.entries if not e["note"] == "void")
