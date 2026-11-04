import { useCallback, useEffect, useMemo, useState } from "react";

export function useCounter(initial: number, step = 1, max = 10) {
  const [count, setCount] = useState(initial);
  const [history, setHistory] = useState<number[]>([]);

  const increment = useCallback(() => {
    setCount((c) => (c + step > max ? max : c + step));
  }, [step, max]);

  const reset = () => setCount(initial);

  const parity = useMemo(() => (count % 2 === 0 ? "even" : "odd"), [count]);

  useEffect(() => {
    document.title = `count: ${count}`;
    if (count === max) setHistory((h) => [...h, count]);
  }, [count, max]);

  return { count, parity, history, increment, reset };
}
