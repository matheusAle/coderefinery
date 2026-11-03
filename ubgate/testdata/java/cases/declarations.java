package demo.decl;

import static java.lang.Math.*;
import java.lang.annotation.*;

@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.METHOD, ElementType.TYPE})
@interface Marker {
    String value() default "";
    int[] codes() default {};
    Class<?> type() default Object.class;
    Level level() default @Level(1);
    int CONST = 1;
    enum Kind { A, B }
    ;
}

@interface Level { int value(); }

public sealed abstract class Declarations<T extends Comparable<T> & java.io.Serializable>
        extends Base<T> implements Runnable, java.io.Closeable permits Declarations.Sub {
    public static final long serialVersionUID = 1L;
    private transient volatile int counter;
    protected int[] arr[] = new int[0][];
    native void nat();
    strictfp double sd() { return 0; }
    public abstract <R> R accept(Visitor<R> v);
    @SafeVarargs
    final <E> void varargs(E... es) {}
    int dims()[] { return null; }
    void recv(Declarations<T> this, int x) {}
    public <U> Declarations(U u) { super(); }
    Declarations() { this(null); }
    public void run() {}
    public void close() throws java.io.IOException, RuntimeException {}

    static non-sealed class Sub extends Declarations<String> {
        Sub() { super((Object) null); }
        public <R> R accept(Visitor<R> v) { return v.visit(this); }
    }

    interface Visitor<R> {
        R visit(Sub s);
        default R fallback() { return null; }
        static <R> Visitor<R> of() { return null; }
        private void helper() {}
        int CONST = 3, OTHER = 4;
    }

    enum Op implements java.util.function.IntBinaryOperator {
        PLUS("+") {
            public int applyAsInt(int a, int b) { return a + b; }
        },
        @Deprecated MINUS("-") {
            public int applyAsInt(int a, int b) { return a - b; }
        };

        private final String sym;
        Op(String sym) { this.sym = sym; }
    }

    enum Empty {}
    enum Trailing { A, B, }
    enum OnlySemi { ; static int x; }

    record Point(int x, int y) implements Comparable<Point> {
        static final Point ORIGIN = new Point(0, 0);
        Point {
            if (x < 0 || y < 0) throw new IllegalArgumentException();
        }
        Point(int both) { this(both, both); }
        public int compareTo(Point o) { return Integer.compare(x * x + y * y, o.x * o.x + o.y * o.y); }
    }

    record Gen<T>(@Ann T value, java.util.List<T>... rest) {}

    class Inner {
        class Deeper {
            void deep() {}
        }
    }

    static {
        System.loadLibrary("x");
    }

    {
        counter = 0;
    }
}
