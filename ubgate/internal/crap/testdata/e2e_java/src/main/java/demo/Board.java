package demo;

import java.util.List;

public class Board {
    private final List<Integer> cells;

    Runnable onReset = new Runnable() {
        @Override
        public void run() {
            if (cells.isEmpty()) {
                return;
            }
        }
    };

    Board(List<Integer> cells) {
        if (cells == null) {
            throw new IllegalArgumentException();
        }
        this.cells = cells;
    }

    @Deprecated
    public int place(int x) {
        if (x > 0 && x < cells.size()) {
            return cells.get(x);
        }
        return -1;
    }

    public int place(String name) {
        return name.isEmpty() ? 0 : place(name.length());
    }

    int score() {
        int total = 0;
        for (int cell : cells) {
            total += switch (cell) {
                case 0 -> 0;
                case 1, 2 -> 1;
                default -> cell > 10 ? 3 : 2;
            };
        }
        return total;
    }

    static class Cell {
        int value;

        boolean empty() {
            return value == 0 || value < 0;
        }
    }

    enum Mode {
        FAST {
            int speed() { return 2; }
        },
        SLOW;

        int speed() { return 1; }
    }
}
