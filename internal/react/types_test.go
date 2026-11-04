package react

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// project writes files under a fresh root and returns it.
func project(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// propsOf analyses the named files of a project in one run (one resolver)
// and returns every component's props by name.
func propsOf(t *testing.T, root string, names ...string) map[string]Props {
	t.Helper()
	var files []string
	for _, n := range names {
		files = append(files, filepath.Join(root, n))
	}
	units, _, err := Analyze(files, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Props{}
	for _, u := range units {
		if u.Kind == KindComponent {
			out[u.Name] = u.Props
		}
	}
	return out
}

func expectProps(t *testing.T, got map[string]Props, want map[string]Props) {
	t.Helper()
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("no component %s in %v", name, got)
			continue
		}
		g.PassThrough = nil
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s props = %+v, want %+v", name, g, w)
		}
	}
	// smoke: no unexpected components
	if len(got) != len(want) {
		t.Errorf("components = %v, want %d", got, len(want))
	}
}

func TestPropsFromImportedTypes(t *testing.T) {
	root := project(t, map[string]string{
		"src/types.ts": `
export interface Base { id: string; hidden?: boolean }
export interface CardProps extends Base { title: string; onOpen(): void }
type Local = { a: number };
export type { Local as Renamed };
export default interface DefaultProps { d: boolean }
`,
		"src/index.ts": `
export * from "./types";
export { default as DefProps } from "./types";
export { Theme as ThemeProps } from "./theme";
`,
		"src/theme.ts": `export type Theme = { dark: boolean; accent?: string };`,
		"src/App.tsx": `
import type { CardProps } from "./types";
import { type Renamed as R } from "./types";
import D from "./types";
import { DefProps, ThemeProps, Base } from ".";
import * as T from "./types";
export function Card(props: CardProps) { return <div />; }
export const Renamed = ({ a }: R) => <i />;
export const Def = (p: D) => <i />;
export const ViaIndex = (p: DefProps & ThemeProps & Base) => <i />;
export const Qualified = (p: T.CardProps) => <i />;
`,
	})
	expectProps(t, propsOf(t, root, "src/App.tsx"), map[string]Props{
		"Card":      {Count: 4, Boolean: 1, Optional: 1, Callback: 1},
		"Renamed":   {Count: 1},
		"Def":       {Count: 1, Boolean: 1},
		"ViaIndex":  {Count: 5, Boolean: 3, Optional: 2},
		"Qualified": {Count: 4, Boolean: 1, Optional: 1, Callback: 1},
	})
}

func TestPropsTypeOperators(t *testing.T) {
	root := project(t, map[string]string{
		"src/base.ts": `
export interface Field { name: string; label?: string; disabled: boolean; onChange: (v: string) => void; size: number }
export type Keys = "size" | "label";
`,
		"src/F.tsx": `
import { Field, Keys } from "./base";
type Variant = { kind: "a"; a: number } | { kind: "b"; b?: boolean } | undefined;
type Box<T = { w: number }> = T & { h: number };
export const Omitted = (p: Omit<Field, "size" | "label">) => <i />;
export const OmitAlias = (p: Omit<Field, Keys>) => <i />;
export const Picked = (p: Pick<Field, "name" | "disabled">) => <i />;
export const OmitKeyof = (p: Omit<Field, keyof { name: 1; size: 2 }>) => <i />;
export const Opt = (p: Partial<Field>) => <i />;
export const Req = (p: Required<Readonly<Field>>) => <i />;
export const Children = (p: React.PropsWithChildren<{ x: number }>) => <i />;
export const Union = (p: Variant) => <i />;
export const Defaulted = (p: Box) => <i />;
export const Given = (p: Box<{ d: boolean }>) => <i />;
`,
	})
	expectProps(t, propsOf(t, root, "src/F.tsx"), map[string]Props{
		"Omitted":   {Count: 3, Boolean: 1, Callback: 1},
		"OmitAlias": {Count: 3, Boolean: 1, Callback: 1},
		"Picked":    {Count: 2, Boolean: 1},
		"OmitKeyof": {Count: 3, Boolean: 1, Optional: 1, Callback: 1},
		"Opt":       {Count: 5, Boolean: 1, Optional: 5, Callback: 1},
		"Req":       {Count: 5, Boolean: 1, Callback: 1},
		"Children":  {Count: 2, Optional: 1},
		"Union":     {Count: 3, Boolean: 1, Optional: 2}, // kind in both; a, b only in one
		"Defaulted": {Count: 2},
		"Given":     {Count: 2, Boolean: 1},
	})
}

func TestUnresolvablePropsAreOpenAndOnlyKnownMembersCount(t *testing.T) {
	root := project(t, map[string]string{
		"src/O.tsx": `
import type { ComponentProps, HTMLAttributes } from "react";
import { Missing } from "./missing";
type Mapped = { [K in "a" | "b"]: string };
type Indexed = { a: string; [key: string]: unknown };
export const Lib = (p: ComponentProps<"button"> & { variant: string }) => <i />;
export const Dom = ({ id }: React.HTMLAttributes<HTMLDivElement>) => <i />;
export const PickLib = (p: Pick<HTMLAttributes<HTMLElement>, "title" | "onClick">) => <i />;
export const OmitLib = (p: Omit<HTMLAttributes<HTMLElement>, "title"> & { own?: boolean }) => <i />;
export const NoFile = (p: Missing) => <i />;
export const MappedT = (p: Mapped) => <i />;
export const IndexT = (p: Indexed) => <i />;
export const Cond = (p: string extends "x" ? { a: 1 } : { b: 2 }) => <i />;
export const AnyT = ({ a, b }: any) => <i />;
export function Generic<T>(p: T & { k: string }) { return <i />; }
`,
	})
	expectProps(t, propsOf(t, root, "src/O.tsx"), map[string]Props{
		"Lib":     {Count: 1, Open: true},
		"Dom":     {Count: 1, Open: true},
		"PickLib": {Count: 2, Callback: 1, Open: true},
		"OmitLib": {Count: 1, Boolean: 1, Optional: 1, Open: true},
		"NoFile":  {Open: true},
		"MappedT": {Open: true},
		"IndexT":  {Count: 1, Open: true},
		"Cond":    {Open: true},
		"AnyT":    {Count: 2, Open: true},
		"Generic": {Count: 1, Open: true},
	})
}

func TestCyclesAndSelfReferencesTerminateOpen(t *testing.T) {
	root := project(t, map[string]string{
		"src/a.ts": `export * from "./b"; export interface A extends B2 { a: string }; import { B2 } from "./b";`,
		"src/b.ts": `export * from "./a"; export interface B2 extends A2 { b: string }; import { A } from "./a"; type A2 = A;`,
		"src/C.tsx": `
import { A, Nowhere } from "./a";
type Self = Self & { s: number };
export const Cyc = (p: A) => <i />;
export const Star = (p: Nowhere) => <i />;
export const Selfish = (p: Self) => <i />;
`,
	})
	got := propsOf(t, root, "src/C.tsx")
	expectProps(t, got, map[string]Props{
		// A -> B2 -> A2 = A: the cycle back to A is open; a and b are still known
		"Cyc":     {Count: 2, Open: true},
		"Star":    {Open: true},
		"Selfish": {Count: 1, Open: true},
	})
}

func TestTsconfigPathsAndBaseURL(t *testing.T) {
	root := project(t, map[string]string{
		"tsconfig.json": `{
  // comments and trailing commas are allowed
  "compilerOptions": { "baseUrl": ".", "paths": { "@ui/*": ["./packages/ui/src/*"], "@ui": ["./packages/ui/src/index.ts"], }, },
}`,
		"packages/ui/src/index.ts":   `export type { ButtonProps } from "./button";`,
		"packages/ui/src/button.ts":  `export interface ButtonProps { label: string; primary?: boolean }`,
		"packages/app/tsconfig.json": `{ "extends": "../../tsconfig.json" }`,
		"packages/app/src/shared.ts": `export type Shared = { s: number };`,
		"packages/app/src/App.tsx": `
import type { ButtonProps } from "@ui";
import type { ButtonProps as Deep } from "@ui/button";
import type { Shared } from "packages/app/src/shared";
import type { Gone } from "@ui/gone";
export const ViaPaths = (p: ButtonProps) => <i />;
export const ViaWildcard = (p: Deep) => <i />;
export const ViaBaseUrl = (p: Shared) => <i />;
export const NotFound = (p: Gone) => <i />;
`,
	})
	expectProps(t, propsOf(t, root, "packages/app/src/App.tsx"), map[string]Props{
		"ViaPaths":    {Count: 2, Boolean: 1, Optional: 1},
		"ViaWildcard": {Count: 2, Boolean: 1, Optional: 1},
		"ViaBaseUrl":  {Count: 1},
		"NotFound":    {Open: true},
	})
}

func TestModulesAreParsedOncePerRun(t *testing.T) {
	root := project(t, map[string]string{
		"src/p.ts":              `export type P = { a: number };`,
		"src/A.tsx":             `import { P } from "./p"; export const A = (p: P) => <i />;`,
		"src/B.tsx":             `import { P } from "./p.js"; export const B = (p: P) => <i />;`,
		"src/C.jsx":             `export const C = ({ c }) => <i />;`,
		"src/D.tsx":             `import { P } from "../../outside/p"; export const D = (p: P) => <i />;`,
		"src/E.tsx":             `import { Q } from "./node_modules/q"; export const E = (p: Q) => <i />;`,
		"src/node_modules/q.ts": `export type Q = { q: number };`,
	})
	r := newResolver(root)
	var props []Props
	for _, name := range []string{"src/A.tsx", "src/B.tsx", "src/C.jsx", "src/D.tsx", "src/E.tsx"} {
		text, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		f, err := r.analyzeSource(string(text), name, root)
		if err != nil || len(f.Units) != 1 {
			t.Fatalf("%s: %v %+v", name, err, f.Units)
		}
		props = append(props, f.Units[0].Props)
	}
	want := []Props{{Count: 1}, {Count: 1}, {Count: 1}, {Open: true}, {Open: true}}
	for i := range want {
		props[i].PassThrough = nil
	}
	if !reflect.DeepEqual(props, want) {
		t.Fatalf("props = %+v, want %+v", props, want)
	}
	// one cached module per file read: the five analysed files and p.ts
	cached := 0
	for path, m := range r.files {
		if m != nil {
			cached++
			if m.path != path {
				t.Errorf("module %s cached as %s", m.path, path)
			}
		}
	}
	if cached != 6 {
		t.Fatalf("cached modules = %d, want 6", cached)
	}
}

func TestStripJSONC(t *testing.T) {
	got := string(stripJSONC([]byte(`{"a": "x // not a comment", /* c */ "b": [1, 2,], // tail
}`)))
	want := `{"a": "x // not a comment",   "b": [1, 2]}` // whitespace before a closer goes with the comma
	if got != want {
		t.Fatalf("stripJSONC = %q, want %q", got, want)
	}
}
