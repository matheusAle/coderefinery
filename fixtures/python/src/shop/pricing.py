"""Cart pricing: classes, private helpers, comprehensions, match/case."""


TAX_RATE = 0.1


class Cart:
    def __init__(self, owner):
        self.owner = owner
        self.items = []

    def add(self, name, price, qty=1):
        if price < 0 or qty <= 0:
            raise ValueError("bad item")
        self.items.append((name, price, qty))
        return len(self.items)

    def subtotal(self):
        return sum(price * qty for _, price, qty in self.items)

    def expensive(self, limit):
        return [name for name, price, _ in self.items if price > limit]

    def total(self, member=False):
        base = self.subtotal()
        discount = self._discount(base, member)
        return round((base - discount) * (1 + TAX_RATE), 2)

    def _discount(self, base, member):
        rate = 0.05 if member else 0.0
        if base >= 100 and member:
            rate = rate + 0.05
        return base * rate


def shipping(country, weight):
    match country:
        case "US":
            return 5 if weight < 10 else 12
        case "CA" | "MX":
            return 9
        case _:
            return 20 + weight * 2


def parse_quantity(text):
    try:
        value = int(text)
    except ValueError:
        return 0
    except TypeError:
        return -1
    return value if value > 0 else 0


def eligible(customer):
    return customer.get("active") and (customer.get("age", 0) >= 18 or customer.get("guardian"))


def bucket_totals(orders):
    def key(order):
        return "big" if order["amount"] > 50 else "small"

    totals = {}
    for order in orders:
        if order.get("void"):
            continue
        totals[key(order)] = totals.get(key(order), 0) + order["amount"]
    return totals
