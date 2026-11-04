class G<T extends Comparable<T>> { <U> U m(java.util.List<? extends U> l) { return l.isEmpty() ? null : l.get(0); } }
