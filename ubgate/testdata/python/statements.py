import os


@decorator
@dec.with_args(1, 2)  # deco comment
def f(a, b: int, c=1, d: str = "x", *args, e, **kwargs) -> bool:
    """Doc."""
    if a:
        pass
    elif b:
        pass
    else:
        pass
    for i in range(3):
        continue
    else:
        pass
    while x:
        break
    else:
        pass
    try:
        pass
    except ValueError as e:
        pass
    except (A, B):
        pass
    except:
        pass
    else:
        pass
    finally:
        pass
    try:
        pass
    except* TypeError:
        pass
    with open(x) as fh, open(y) as gh:
        pass
    with (open(x) as fh, open(y)):
        pass
    async with a:
        pass
    return a if b else c


async def g():
    async for x in y:
        await x


class A(B, metaclass=M):
    x = 1

    def m(self):
        return 1

    @property
    def p(self): return 2


class C:
    pass


class D[T](Base):
    def h[U](self, u: U) -> T: ...


def outer():
    def inner():
        class Inner:
            def deep(self):
                pass
        return Inner
    return inner


match command:
    case [x, y, *rest]:
        pass
    case {"a": 1, **kw}:
        pass
    case Point(x=0, y=1) if x > 0:
        pass
    case 1 | 2 | 3:
        pass
    case -1:
        pass
    case 1 + 2j:
        pass
    case "s" "t":
        pass
    case str() as s:
        pass
    case (a, b):
        pass
    case a.b.c:
        pass
    case None | True | False:
        pass
    case _:
        pass
if x: pass
while y: x = 1; y = 2
for a, b in c: pass
for (a, b) in c: pass
def one(): return 1
class E: pass
