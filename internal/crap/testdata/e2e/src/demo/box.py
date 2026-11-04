def choose(x, ready):
    if x > 0 and x < 10:
        return 1
    elif ready:
        return 0
    return x


class Box:
    def open(self, flag=False):
        return 1 if flag else 0
