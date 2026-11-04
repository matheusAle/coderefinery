import os


class Box:
    def alpha(self, xs, *args, key=None, **kw):
        ys = [x for x in xs if x > 0]
        zs = {k: v for k, v in kw.items()}
        total = sum(x * 2 for x in ys)  # trailing comment
        return f"{total!r:>10} {len(zs)}", (len)(xs), xs.filter(odd), -total

    async def beta(self, items: list[int] | None = None) -> dict[str, int]:
        async with self.lock as held, other():
            await held.wait()
        try:
            value = items[1:2] if items else not items
        except (KeyError, ValueError) as exc:
            raise RuntimeError("x") from exc
        finally:
            del value
        match items:
            case [1, *rest] | {"k": rest}:
                pass
            case Point(x=0) if rest:
                return {"a": 1}
        lambda q: q is not None and q not in items
        global os
        return None

    # a comment at class indentation


def gamma(a, /, b, *, c):
    def inner():
        nonlocal a
        a @= b ** c // 2
    while a < b <= c:
        a, *b = b
        yield from inner()
    assert a, "msg"
    for i in range(10):
        continue
    else:
        log(i, *a.b())
    return a << 1 | b >> 2 & ~c ^ a
