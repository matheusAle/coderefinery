export function Badge(props: { name: string; admin?: boolean; count?: number }) {
  return (
    <span className="badge">
      {props.name}
      {props.admin && <strong>admin</strong>}
      {(props.count ?? 0) > 9 ? "9+" : props.count}
    </span>
  );
}
