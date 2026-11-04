"""Pricing with arms the tests never take (crap --branches e2e)."""


def tier_rate(tier):
    if tier == "gold":
        return 0.8
    elif tier == "silver":
        return 0.9
    return 1.0


def price(items, tier="basic"):
    total = 0
    for unit, qty in items:
        total += unit * qty
    if total <= 0:
        return 0
    return round(total * tier_rate(tier), 2)

def receipt(items):
    for item in items:
        print(item)
    return len(items)


def shipping(country):
    match country:
        case "US":
            return 5
        case "CA" | "MX":
            return 9
        case _:
            return 20


def unused(flag):
    if flag:
        return 1
    return 0
