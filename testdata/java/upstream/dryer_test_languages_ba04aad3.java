class Right {
  int beta(int[] items) {
    int[] kept = filter(items, even);
    int[] extra = filter(kept, even);
    return map(extra, dec);
  }
}
