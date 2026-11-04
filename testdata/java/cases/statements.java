import java.util.*;

public class Statements {
    int all(int n, List<String> xs) throws Exception {
        ;
        {}
        { int scoped = 1; }
        final int a = 1, b[] = {}, c;
        var list = new ArrayList<String>();
        int[] d[], e;
        java.util.Map<String, List<int[]>> m = new HashMap<>();
        Map.Entry<String, Integer> entry = null;
        label: while (true) { break label; }
        if (n > 0) return 1;
        if (n > 1) { n--; } else if (n > 2) { n++; } else { n = 0; }
        for (;;) { break; }
        for (int i = 0; i < n; i++) ;
        for (i = 0, j = 1; i < j; i++) {}
        for (String x : xs) {}
        for (@SuppressWarnings("x") final var x : xs) {}
        for (int[] row : grid) {}
        for (Map.Entry<String, Integer> en : m2.entrySet()) {}
        while (n > 0) n--;
        do n++; while (n < 10);
        try { f(); } catch (Exception e) { g(); }
        try { f(); } finally { g(); }
        try { f(); } catch (final IOException | RuntimeException e) { } catch (Throwable t) { } finally { }
        try (in) { }
        try (var in = open(); var out = open();) { }
        try (final InputStream in = open()) { } catch (IOException e) {}
        switch (n) {
            case 1:
            case 2: {
                break;
            }
            case 3:
                n++;
                // fallthrough
            case 4:
                n--;
                break;
            default:
                break;
        }
        switch (n) { }
        switch (s) { case "a" -> f(); case "b" -> { g(); } default -> throw new X(); }
        switch (o) {
            case null -> f();
            case String s when s.length() > 1 && s.isBlank() -> g();
            case Integer i -> {}
            default -> {}
        }
        switch (e) { case A, B -> f(); case C -> g(); }
        throw new RuntimeException("x");
    }

    void more() {
        synchronized (lock) { x(); }
        assert x;
        assert x != null : "msg" + x;
        return;
    }

    int yields(int k) {
        int yield = 1;
        yield = 2;
        yield++;
        return switch (k) {
            case 1:
                yield 1;
            case 2: {
                yield (2);
            }
            default:
                yield -1;
        };
    }

    void locals() {
        class Local extends Base implements I { void run() {} }
        abstract class Abs {}
        final class Fin {}
        record Pair<A, B>(A a, B b) {}
        enum Color { RED, GREEN }
        interface Loc { void x(); }
        Local l = new Local();
        l.run();
    }

    void labels() {
        outer:
        for (int i = 0; i < 3; i++) {
            inner:
            for (int j = 0; j < 3; j++) {
                if (j == 1) continue outer;
                if (j == 2) break inner;
            }
        }
    }
}
