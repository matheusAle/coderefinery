class Left {
  int alpha(int[] xs) {
    int[] ys = filter(xs, odd);
    return map(ys, inc);
  }
}
