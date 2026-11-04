a + b * c - d / e // f % g @ h
-x ** 2
2 ** -1 ** 3
a << 1 >> 2 & 3 | 4 ^ 5
~a + +b
not a < b
a < b <= c == d != e > f >= g
a in b not in c is d is not e
a <> b
a and b or c and not d
x if y else z if w else v
lambda: 0
lambda x, *args, y=1, **kw: x
lambda *, k: k
lambda x, /: x
await foo()
(y := f(x))
f(a, *b, c=1, **d)
f(x for x in y)
f(x for x in y if x)
obj.attr.sub[0][1:2, ::3, 4:][...]
a[1:2:3]
a[:]
a[b, c]
[1, 2, *rest]
[]
()
(1,)
(1, 2)
{}
{1, 2}
{'a': 1, **other}
[x for x in y if x for z in x]
{k: v for k, v in items}
{x for x in y}
(x async for x in y)
1; 2; 3;
a = b = c
a, b = b, a
a, = x
*a, b = c
(a, b) = c
[a, b] = c
a.b = 1
a[0] = 1
x: int = 5
y: list[int]
a += 1
a //= 2
a **= 3
a @= b
a >>= 1
x = yield
x = yield from y
x = yield a, b
print(x)
print >>sys.stderr, "x"
print "hello"
exec "code" in ns
True, False, None, ...
0x1F, 0o17, 0b101, 1_000, 10L, 1.5, 1e10, 1.5j, .5, 5.
'single' "double" '''triple''' """triple2"""
b'bytes' r'raw\n' rb'x' u'uni' Rb"y"
'esc \n \t \\ \' \x41 ሴ \N{DASH} \777 \q'
f'{a} {b!r} {c:>10} {d=} {e!s:{w}.{p}}'
f'{{literal}} {x:{{}}}'
f"{'nested'}" f'{f"{x}"}'
f'''multi
{x}
line'''
foo(
    a,  # comment
    b,
)
x = 1 + \
    2
del a, b
del (a)
assert x, "msg"
raise
raise E
raise E from e
return
global a, b
nonlocal c
pass
break
continue
import a
import a.b.c as d, e
from . import x
from .. import (y, z,)
from .m import *
from a.b import c as d
from __future__ import annotations
type X = int
type Y[T] = list[T]
match = 1
match(x)
type = 3
print = 4
x = a.match
