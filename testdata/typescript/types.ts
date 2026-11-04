function f<T extends object = {}>(a: number, b?: string, ...c: any[]): void {}
let x: A.B<C>[] = [];
type U = string | number & boolean;
type F = (a: number) => void;
type N = new (a: number) => X;
type K = keyof T;
type Q = typeof x.y;
type L = T["a"];
type C = T extends (infer U)[] ? U : never;
type M = { readonly [K in keyof T]?: T[K] };
type TL = `a${B}c`;
type Tu = [a: string, b?: number, ...rest: any[]];
type O = { a: number; b?(): void; [k: string]: any; new (): X; (): Y };
type P = -1 | "s" | true | null | undefined;
interface I extends J<K>, L { m(x: number): void; readonly p: string }
enum E { A = 1, B, "c" }
declare module "m" { export const z: number; }
namespace NS.Inner { }
abstract class AC<T> extends B<T> implements I, J { private readonly x: number = 1; abstract m(): void; constructor(public y: string) { super(); } }
let v = x as unknown as T;
let w = y satisfies Z;
let nn = a!.b!;
let ta = <T>z;
function g(this: Foo, x): x is string { return true }
function h(x): asserts x is string {}
let ie = f<string>;
export = foo;
import fs = require("fs");
import type { A } from "a";
export type { B } from "b";
let fnt: () => void;
let u: unique symbol;
