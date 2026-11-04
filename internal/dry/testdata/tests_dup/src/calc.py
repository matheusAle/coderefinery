def total(items):
    result = 0
    for price, qty in items:
        if qty > 0:
            result += price * qty
    return result


def weight(items):
    result = 0
    for grams, qty in items:
        if qty > 0:
            result += grams * qty
    return result
