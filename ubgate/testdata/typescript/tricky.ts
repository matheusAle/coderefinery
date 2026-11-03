#!/usr/bin/env node
import type { A, B as C } from "./a";
import D, { type E, F } from "./d";
import * as NS from "ns";
import "side-effect";
import json from "./x.json" with { type: "json" };
export * from "./all";
export * as ns2 from "./all";
export { A as default, F };
export type { C };
export default interface Def { x: number }
let re = /ab+c/i, div = a / b / c;
if (x) /re/.test(s);
const g = (a) / 2;
const t = `a${`b${c}`}d\`${e}`;
let o = { if: 1, class: 2, "str": 3, 4: 5, [k]: 6, get: 7, set: 8, async: 9, static: 10, new: 11 };
o.if = o.class + o.new;
label: for (const x of y) { if (x) continue label; else break label; }
let async = 1; async = async + 1;
let of = 2, get = 3, set = 4, type = 5, declare = 6, abstract = 7, readonly = 8, module = 9, namespace = 10;
type = declare + abstract;
const fn = async <T,>(x: T): Promise<T> => x;
const fn2 = <T>(x: T) => x;
const fn3 = async x => x;
const fn4 = (x: number, ...rest: string[]): x is number => true;
const fn5 = ({ a, b }: { a: number; b: string }, [c, d]: [number, string] = [1, "x"]) => a;
let cmp = a < b > c;
let gen = f<T>(x), gen2 = new Map<string, number[]>(), inst = f<T>;
let x2 = y! + z!.w!;
let casted = <HTMLElement>document.body;
let sat = { a: 1 } satisfies Record<string, number>;
let pc = a?.[0]?.b?.(c)?.d;
let tpl = tag`x${y}z`;
let n1 = 0x1F, n2 = 1_000_000n, n3 = .5e-3, n4 = 0b1010, n5 = 0o17, n6 = 1e10;
class Foo<T extends object = {}> extends Bar<T> implements Baz, Qux<T> {
  static readonly #count: number = 0;
  private declare x?: string;
  protected abstract y: number;
  public override z!: number;
  [key: string]: any;
  static { Foo.init(); }
  constructor(private readonly a: string, public b = 1) { super(); }
  get value(): number { return this.#v; }
  set value(v: number) { this.#v = v; }
  async *items(): AsyncGenerator<number> { yield 1; }
  method<U>(this: Foo<T>, u: U): asserts this is Foo<U> {}
  optional?(): void;
  @decorator() decorated() {}
  @a.b.c @d(1, 2) prop = 1;
  #priv() { return #v in this; }
}
abstract class Abs { abstract m(): void; protected abstract get p(): number; }
interface Ifc<T> extends Base<T>, Other {
  (x: number): string;
  new (x: number): Ifc<T>;
  readonly [index: number]: string;
  method?<U>(u: U): void;
  prop: T;
  get acc(): number;
}
type Mapped<T> = { readonly [K in keyof T as `get${Capitalize<K & string>}`]-?: () => T[K] };
type Cond<T> = T extends (infer U)[] ? U : T extends Promise<infer V> ? V : never;
type Tup = [first: string, second?: number, ...rest: boolean[]];
type Fn = <T>(x: T) => T;
type Ctor = abstract new () => object;
type KT = keyof typeof obj;
type Lit = "a" | 'b' | 1 | -1 | true | null | undefined | `t${string}`;
type Pred = (x: unknown) => x is string;
type Asserts = (x: unknown) => asserts x;
type Ix = Foo["bar"][number];
type Obj = { a: string, b?: number; c(): void, readonly d: boolean };
type Paren = (A | B)[];
declare module "mod" { export function f(): void; }
declare global { interface Window { x: number } }
declare namespace DN { const y: number; }
declare const dc: number;
declare let dl: string;
export declare function edf(): void;
export abstract class EAC {}
export enum EE { A = 1 << 0, B = A | 2, C = "c".length }
export const enum CE { X }
export namespace ENS.Inner { export const z = 1; }
export import Alias = NS.Inner;
import req = require("req");
export = Foo;
function overload(a: string): void;
function overload(a: number): void;
function overload(a: any) {}
let definite!: number;
let [aa, , bb = 2, ...cc] = arr;
({ aa, bb: { cc } = {}, ...dd } = obj);
for (const [k, v] of Object.entries(o)) {}
for (let i = 0, j = 10; i < j; i++, j--) {}
for (const k in o) if (k) delete o[k];
while (i--) ;
do i++; while (i < 10)
switch (k) { case "a": { break; } default: }
try { throw new Error() } catch ({ message }) { } finally { }
x = a ? b : c ? d : e;
x = a ?? b ?? c;
x = a || b && c | d ^ e & f == g < h << i + j * k ** l;
x = -a ** 2;
x = typeof a === "string" && !b;
x = void 0, delete y.z, typeof w;
x ||= 1; x &&= 2; x ??= 3; x **= 2; x >>>= 1;
x = new Foo;
x = new Foo.Bar<number>(1).baz;
x = new new Foo()();
x = import.meta.url;
x = await import("./m");
function* g2() { const r = yield* other(); yield; }
async function af() { for await (const x of xs) await x; }
x = function named() {};
x = class Named {};
x = (a, b) => a + b;
x = async () => { await 1; };
x = a
++b
x = a
(b)
x = `multi
line ${a +
b}`;
