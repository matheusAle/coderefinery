abstract class Left {
  abstract int declared(int[] xs);
  int alpha(int[] xs) {
    class Inner {
      int run(int[] ys) {
        return ys.length;
      }
    }
    return xs.length;
  }
}
