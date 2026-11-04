x = {a: 1, b, [c]: 2, ...d, e() {}, get f() {}, set g(v) {}, async h() {}, *i() {}, async *j() {}, "k": 1, 2: 3, get: 1, async: 2, static: 3};
({a, b: [c], ...d} = e);
[a, , b = 1, ...c] = d;
f = async x => x;
f = async (a, {b}, [c] = [], ...d) => {};
f = function* g(a = 1) { yield; yield* a; yield b };
new.target; import.meta; import("x");
`a${b}c${`d${e}`}\n`;
"a\nbA\x41\
c";
'it\'s';
/ab[/]c/gi.test(s);
a ? b : c;
a, b;
typeof a; void 0; delete a.b; !a; ~a; -a; +a; ++a; a--; await a;
a.#b; #b in c;
class A extends B { static x = 1; #y; static { z() } static get w() {} 'q'() {} [k] = 1; static async *m() {} }
@dec class D {}
tag`x`;
a?.b.c?.[d]?.(e);
x **= 2; x ??= 1; x >>>= 1;
