export function choose(x: number): number {
  if (x > 0 && x < 10) return 1;
  for (let i = 0; i < x; i++) {
    if (i === 2) return i;
  }
  try {
    return x;
  } catch (e) {
    return 0;
  }
  return x > 0 ? 1 : 0;
}

export class Box {
  open(flag?: boolean): number {
    return flag ? 1 : 0;
  }
}

const arrow = (n: number) => (n > 0 ? n : 0);

function outer() {
  function inner(n: number) {
    if (n) return 1;
    return 0;
  }
  return inner(1);
}
export function choose2(a, b, c) {
  return a ?? b?.c ?? c?.() ?? c?.[0];
}
export function text() {
  return "a ?? b?.c";
}
export function mount(app) {
  const extra = [1].map((n) => (n ? 1 : 0));
  app.get("/users", (req, res) => {
    if (req.query.q) return 1;
    return 0;
  });
  app.post("/users", function (req, res) {
    return req.body ?? {};
  });
  app.use((req, res, next) => next());
  app.route("/items").get((req, res) => res.send(1)).post((req, res) => res.send(2));
  return extra;
}
app.get("/health", async (req, res) => res.send(req.query.ok ?? "no"));
app.get("/users", (req, res, next) => next(), (req, res) => res.send(req.id ?? 0));
app.delete(`/users/${id}`, ((req, res) => res.send(1)));
function* gen() { if (a) {} }
async function af() { if (a) {} }
var v = () => { if (a) {} };
let l = function () { if (a) {} }, m = async (x) => x ? 1 : 2;
const K = class { m() { if (a) {} } };
class A { m() { return { n() { if (a) {} } }; } }
function f(a) { return a?.b.c?.d; }
function g(a) { a ??= 1; a ||= 2; a &&= 3; return a; }
class S { async run() { router.get("/x", (req) => { if (a) {} }); } }
app.get("/a\nb", (r) => 1);
app.get('/q', (r) => 1); app.get(path, handler); app.get(handler, "/late", function () {}); app.GET("/up", () => 1); cache.get("k", () => 1);
function f2(xs) { for (const x of xs) {} for (const k in xs) {} for await (const y of xs) {} label: while (1) { break label; } do {} while (0); switch (a) { case 1: case 2: break; default: } }
class A2 { get x() { return a ? 1 : 2; } set x(v) { if (v) {} } static s() {} #p() { if (a) {} } "q"() {} 1() {} }
const {a1, b1} = obj;
const f3 = (() => 1);
const g3 = memo(() => a ? 1 : 2);
if (x) { const h = () => a ? 1 : 2; }
function o() { function i() { if (a) {} } const k = () => b ? 1 : 2; return class { m() { if (c) {} } }; }
function f4() { return a
  ? b
  : c; }
namespace N { export function f() { if (a) {} } }
abstract class B { abstract m(): void; n(): void { if (a) {} } }
interface I { m(): void }
enum E { A }
type T<X> = X extends string ? 1 : 2;
function g5<T>(x: T): x is string { return (x as any)!.y ?? 1; }
class P { handler = () => { if (a) {} }; constructor(private x: number) { if (a) {} } }
declare function d(): void;
function over(a: string): void;
function over(a: any) { if (a) {} }
const a6 = 1 / 2 / 3;
function r(s) { return s.replace(/if|&&|\?\./g, "x") / 2 > 1 ? `t ${a ? `n${b ?? c}` : ""}` : 0; }
export const x7 = { a: () => { if (q) {} } };
export function f8() { const o = { m() { if (q) {} } }; return o; }
app.route("/items").get((req, res) => 1).post((req, res) => 2);
app.route(`/t`).all(() => 1);
app.use("/static", express.static("p"), (req,res) => 1);
app.get("", () => 1);
app.get("/x", async function named(req) { if (a) { app.post("/y", () => 1); } });
export default class { m() { if (a) {} } }
@dec class D { m() { if (a) {} } }
function f9(x?: number, y: string = a ? "1" : "2"): (a?: number) => void { type C<T> = T extends string ? 1 : 2; let q: C<number> | undefined; const o: { a?: number } = {}; return (a?: number) => {}; }
function g10() { return <any>x && y; }
function h10() { return f<string>(x) || g<T, U>(y); }
function k10() { return a < b && c > d; }
