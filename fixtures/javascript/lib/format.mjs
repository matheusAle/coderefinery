export function plural(count, word) {
  return count === 1 ? `${count} ${word}` : `${count} ${word}s`;
}

export const clamp = (value, min, max) => {
  if (value < min) return min;
  if (value > max) return max;
  return value;
};

export function describe(user) {
  const name = user?.name ?? 'anonymous';
  switch (user?.role) {
    case 'admin':
      return `${name} (admin)`;
    case 'guest':
      return `${name} (guest)`;
    default:
      return name;
  }
}

export function totals(rows) {
  let sum = 0;
  let count = 0;
  for (const row of rows) {
    if (row.amount > 0) {
      sum += row.amount;
      count += 1;
    }
  }
  return { sum, count, mean: count === 0 ? 0 : sum / count };
}

export function weights(rows) {
  let total = 0;
  let items = 0;
  for (const row of rows) {
    if (row.weight > 0) {
      total += row.weight;
      items += 1;
    }
  }
  return { total, items, avg: items === 0 ? 0 : total / items };
}
