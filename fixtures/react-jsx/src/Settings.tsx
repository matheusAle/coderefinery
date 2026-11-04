import { useState } from "react";

// Two option rows repeated inside one component.
export function Settings() {
  const [dark, setDark] = useState(false);
  const [compact, setCompact] = useState(true);
  return (
    <form className="settings">
      <label className="row" htmlFor="option">
        <input type="checkbox" checked={dark} onChange={(e) => setDark(e.target.checked)} />
        <span className="title">Dark mode</span>
        <small className="hint">Use a dark theme everywhere</small>
      </label>
      <label className="row" htmlFor="option">
        <input type="checkbox" checked={compact} onChange={(e) => setCompact(e.target.checked)} />
        <span className="title">Compact</span>
        <small className="hint">Show more rows per page</small>
      </label>
      <Badge name="beta" />
    </form>
  );
}

export function Badge({ name }: { name: string }) {
  return (
    <span className="badge">
      <b>{name}</b>
    </span>
  );
}
