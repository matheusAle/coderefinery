package demo.shop;

import java.util.*;
import java.util.stream.Collectors;

sealed interface Shape permits Circle, Square, Shapes.Rect {}

record Circle(double r) implements Shape {}

record Square(double side) implements Shape {}

public class Shapes {
    public static final class Rect implements Shape {
        final double w, h;
        Rect(double w, double h) { this.w = w; this.h = h; }
    }

    static double area(Shape s) {
        return switch (s) {
            case Circle c when c.r() < 0 -> throw new IllegalStateException("negative");
            case Circle c -> Math.PI * c.r() * c.r();
            case Square sq -> sq.side() * sq.side();
            case Rect r -> r.w * r.h;
        };
    }

    static String describe(Object o) {
        if (o instanceof Circle(double r) && r > 1) {
            return "big circle";
        }
        if (!(o instanceof Shape shape)) {
            return "not a shape";
        }
        return switch (shape) {
            case Circle c -> "circle";
            default -> {
                String name = shape.getClass().getSimpleName();
                yield name.isEmpty() ? "anon" : name.toLowerCase();
            }
        };
    }

    static int kind(String day) {
        int n = switch (day) {
            case "SAT", "SUN" -> 0;
            case "MON" -> {
                int x = day.length();
                yield x * 2;
            }
            default -> day.hashCode() & 0xff;
        };
        return n;
    }

    static List<String> names(Collection<? extends Shape> shapes) {
        return shapes.stream()
                .filter(Objects::nonNull)
                .map(s -> s.getClass().getSimpleName())
                .sorted(Comparator.comparing(String::length).thenComparing(Function.identity()))
                .collect(Collectors.toList());
    }

    @SuppressWarnings({"unchecked", "rawtypes"})
    static <T extends Comparable<? super T>> T max(T[] items) {
        T best = null;
        for (final T item : items) {
            best = best == null || item.compareTo(best) > 0 ? item : best;
        }
        return best;
    }

    static int[][] grid(int n) {
        int[][] g = new int[n][];
        for (int i = 0; i < n; i++) {
            g[i] = new int[i + 1];
            Arrays.fill(g[i], i);
        }
        Object[] objs = new String[] {"a", "b"};
        Class<?> k = int[].class;
        Supplier<List<String>> sup = ArrayList::new;
        IntFunction<int[]> mk = int[]::new;
        return g;
    }
}
