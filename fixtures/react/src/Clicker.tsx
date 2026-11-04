import { memo, useState } from "react";

interface LabelProps {
  text: string;
  onSelect: () => void;
  style: { fontWeight: string };
}

// Memoised, but Clicker passes it a new function and object on every render,
// so it re-renders anyway (`react --profile` flags Clicker).
export const Label = memo(function Label({ text, onSelect, style }: LabelProps) {
  return (
    <span style={style} onClick={onSelect}>
      {text}
    </span>
  );
});

// Memoised with no props: re-renders only on its own state.
export const Toggle = memo(function Toggle() {
  const [on, setOn] = useState(false);
  function flip() {
    setOn(!on);
  }
  return <button onClick={flip}>{on ? "on" : "off"}</button>;
});

export function Clicker() {
  const [count, setCount] = useState(0);
  return (
    <div>
      <button onClick={() => setCount(count + 1)}>add</button>
      <Label text="clicks" onSelect={() => setCount(0)} style={{ fontWeight: "bold" }} />
      <Toggle />
      <output>{count}</output>
    </div>
  );
}
