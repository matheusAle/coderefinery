package demo.shop;

import java.io.*;
import java.util.concurrent.atomic.AtomicInteger;

@FunctionalInterface
interface Handler<E extends Exception> {
    void handle(String line) throws E;
}

enum Level {
    LOW(1), MEDIUM(5) {
        @Override
        String label() { return "mid"; }
    }, HIGH(10);

    private final int weight;

    Level(int weight) { this.weight = weight; }

    String label() { return name().toLowerCase(); }

    static Level of(int w) {
        for (Level l : values()) if (l.weight >= w) return l;
        return HIGH;
    }
}

public abstract class Inventory<K, V extends Number> {
    private final AtomicInteger count = new AtomicInteger();
    private static int created;

    static {
        created = 0;
    }

    {
        count.set(0);
    }

    protected abstract V value(K key);

    public synchronized int load(File file, Handler<IOException> h) {
        try (BufferedReader in = new BufferedReader(new FileReader(file));
             Writer out = new StringWriter()) {
            String line;
            while ((line = in.readLine()) != null) {
                h.handle(line);
                count.incrementAndGet();
            }
        } catch (FileNotFoundException | SecurityException e) {
            return -1;
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        } finally {
            created++;
        }
        return count.get();
    }

    long sum(K... keys) {
        long total = 0;
        for (K key : keys) {
            V v = value(key);
            total += v == null ? 0 : v.longValue();
        }
        assert total >= 0 : "overflow";
        return total;
    }

    String report() {
        String header = """
            Inventory report
              count: \{count}
            "quoted" and \"escaped\"
            """;
        StringBuilder sb = new StringBuilder(header);
        synchronized (this) {
            sb.append(count.get()).append('\n');
        }
        Object lock = this;
        int casted = (int) -sb.length() + (int) 'c';
        double ratio = (double) casted / (casted == 0 ? 1 : casted);
        Runnable r = (Runnable & Serializable) () -> System.out.println(ratio);
        r.run();
        return sb.toString();
    }

    class Entry {
        K key;
        V value;

        Entry(K key) {
            Inventory.this.count.incrementAndGet();
            this.key = key;
        }

        V get() { return value != null ? value : Inventory.this.value(key); }
    }

    static class Builder<T> {
        private T seed;

        Builder<T> seed(T seed) {
            this.seed = seed;
            return this;
        }

        <U> Builder<U> map(java.util.function.Function<T, U> f) {
            Builder<U> b = new Builder<>();
            b.seed = f.apply(seed);
            return b;
        }
    }
}
