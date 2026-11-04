/*
 * Shopping cart (corpus for the coderefinery Java parser).
 */
package demo.shop;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import static java.util.Objects.requireNonNull;

/** A cart of line items. */
public final class Cart implements Iterable<Cart.Line> {
    private static final int MAX_LINES = 1_000;
    private final List<Line> lines = new ArrayList<>();
    private final Map<String, Integer> discounts;
    private double total = 0.0d, tax = .5e-1;
    protected long[] history = new long[] {1L, 0x1FL, 0b101, 017};
    private char sep = '\'';

    public Cart(Map<String, Integer> discounts) {
        this.discounts = requireNonNull(discounts, "discounts");
    }

    Cart() {
        this(Map.of());
    }

    // add a line, merging duplicates
    public void add(String sku, int quantity, double price) {
        if (quantity <= 0) {
            throw new IllegalArgumentException("quantity must be positive: " + quantity);
        } else if (lines.size() >= MAX_LINES) {
            return;
        }
        for (Line line : lines) {
            if (line.sku().equals(sku) && line.price() == price) {
                lines.set(lines.indexOf(line), new Line(sku, line.quantity() + quantity, price));
                return;
            }
        }
        lines.add(new Line(sku, quantity, price));
    }

    public double total() {
        double sum = 0;
        for (int i = 0; i < lines.size(); i++) {
            Line line = lines.get(i);
            int off = discounts.getOrDefault(line.sku(), 0);
            sum += line.quantity() * line.price() * (100 - off) / 100.0;
        }
        return sum > 100 ? sum * 0.95 : sum;
    }

    public <R> List<R> map(Function<? super Line, ? extends R> f) {
        List<R> out = new ArrayList<>(lines.size());
        lines.forEach(line -> out.add(f.apply(line)));
        return out;
    }

    @Override
    public java.util.Iterator<Line> iterator() {
        return new java.util.Iterator<>() {
            private int next;

            @Override
            public boolean hasNext() { return next < lines.size(); }

            @Override
            public Line next() {
                if (!hasNext()) throw new java.util.NoSuchElementException();
                return lines.get(next++);
            }
        };
    }

    static String grade(int score) {
        String result;
        switch (score / 10) {
            case 10:
            case 9:
                result = "A";
                break;
            case 8:
                result = "B";
                break;
            default:
                result = score < 0 ? "?" : "C";
        }
        return result;
    }

    int retries(int n) {
        int tries = 0;
        do {
            tries++;
        } while (tries < n && !done(tries));
        while (n-- > 0) {
            if (n % 2 == 0) continue;
            tries += n << 1 >> 1 >>> 0;
        }
        outer:
        for (int i = 0, j = 10; i < j; i++, j--) {
            for (int k = 0; ; k++) {
                if (k > i) break outer;
            }
        }
        return tries;
    }

    private boolean done(int tries) {
        return tries > 3 || tries == -1 ^ false;
    }

    public record Line(String sku, int quantity, double price) {
        public Line {
            if (quantity < 0) {
                throw new IllegalArgumentException();
            }
        }

        double subtotal() { return quantity * price; }
    }
}
