export function View(ok: boolean, ready: boolean) {
  return ok && ready ? 1 : 0;
}
export function V({ok}) { return <div>{ok && <span>a b c</span>}{ok ? <A/> : null}</div>; }
const W = <T,>(x: T) => x ? 1 : 0;
export function Badge(props: { name: string; admin?: boolean; count?: number }) {
  return (
    <span className="badge" data-x='y' {...props}>
      {props.name}
      {props.admin && <strong>admin</strong>}
      {(props.count ?? 0) > 9 ? "9+" : props.count}
      <>frag &amp; ment</>
      <Foo.Bar x={1} y="2" z />
      {/* comment */}
      {list.map((i) => <li key={i}>{i}</li>)}
    </span>
  );
}
const Generic = <T extends object>(p: T): JSX.Element => <div>{String(p)}</div>;
