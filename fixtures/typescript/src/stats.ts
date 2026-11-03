export function sumPositive(values: number[]): { total: number; seen: number } {
  let total = 0;
  let seen = 0;
  for (const value of values) {
    if (value > 0) {
      total += value;
      seen += 1;
    }
  }
  return { total, seen };
}

export function sumWeights(weights: number[]): { total: number; seen: number } {
  let acc = 0;
  let n = 0;
  for (const weight of weights) {
    if (weight > 0) {
      acc += weight;
      n += 1;
    }
  }
  return { total: acc, seen: n };
}
