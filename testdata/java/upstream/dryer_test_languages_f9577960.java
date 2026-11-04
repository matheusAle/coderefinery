class Right {
  int beta(int[] items) {
    int[] kept = filter(items, even);
    return map(kept, dec);
  }
}
