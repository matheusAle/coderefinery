interface BadgeProps {
  label: string;
  counts: number[];
}

export function total(counts: number[]) {
  return counts.reduce((sum, n) => sum + n, 0);
}

export function Badge({ label, counts }: BadgeProps) {
  const count = total(counts);
  return (
    <span className="badge">
      {label} {count > 99 ? "99+" : count}
    </span>
  );
}
