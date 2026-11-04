interface BadgeProps {
  label: string;
  count: number;
}

export function Badge({ label, count }: BadgeProps) {
  return (
    <span className="badge">
      {label} {count > 99 ? "99+" : count}
    </span>
  );
}
