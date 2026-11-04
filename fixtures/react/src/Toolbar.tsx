import type { HTMLAttributes, ReactNode } from "react";

import type { ButtonBase } from "./types";

// Props come from another module, through an index re-export, Omit and &.
type ToolbarProps = Omit<ButtonBase, "variant"> & {
  items: string[];
  selected: string;
  onPick: (item: string) => void;
};

export function Toolbar({ label, items, onPick, ...button }: ToolbarProps) {
  return (
    <nav aria-label={label}>
      {items.map((item) => (
        <button key={item} type="button" disabled={button.disabled} onClick={() => onPick(item)}>
          {item}
        </button>
      ))}
    </nav>
  );
}

// React's DOM attributes are a library type: the props are open (2+).
export function ToolbarGroup({ title, children, ...rest }: { children: ReactNode } & HTMLAttributes<HTMLDivElement>) {
  return (
    <div role="group" {...rest}>
      <h3>{title}</h3>
      {children}
    </div>
  );
}
