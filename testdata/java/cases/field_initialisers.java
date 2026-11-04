package a.b;

public class S {
    Runnable r = new Runnable() {
        public void run() { if (ready) { go(); } }
    };

    enum E {
        ONE {
            int f(int x) { return x > 0 ? 1 : 0; }
        },
        TWO;

        int g() { return 2; }
    }

    @Override
    @Deprecated
    public int h(int v) {
        return switch (v) {
            case 1, 2 -> 1;
            case 3 -> { yield 2; }
            default -> 3;
        };
    }

    interface I {
        void abstractOne();
        default int d(int x) { if (x > 1) return x; return 0; }
    }

    record R(int x, int y) {
        R {
            if (x < 0) throw new IllegalArgumentException();
        }
        int y() { return x > y ? x : y; }
    }

    int l() {
        Runnable inner = () -> {
            if (flag) {
                class Local { void hidden() { if (a) {} } }
            }
        };
        return 1;
    }

    String p(Object o) {
        if (o instanceof String s && !s.isEmpty()) {
            return s;
        }
        return switch (o) {
            case Integer i when i > 0 -> "positive";
            case Integer i -> "int";
            case Point(int px, var py) -> "point";
            default -> "other";
        };
    }
}
