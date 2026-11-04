import { forwardRef, memo, type ReactNode } from "react";

interface IconButtonProps {
  icon: ReactNode;
  title: string;
  pressed?: boolean;
}

export const IconButton = memo(
  forwardRef<HTMLButtonElement, IconButtonProps>(({ icon, ...rest }, ref) => (
    <button ref={ref} type="button" {...rest}>
      {icon}
    </button>
  )),
);
