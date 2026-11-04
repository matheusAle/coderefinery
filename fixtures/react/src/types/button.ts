export interface Pressable {
  disabled?: boolean;
  onPress: () => void;
}

export interface ButtonBase extends Pressable {
  label: string;
  variant: "primary" | "ghost";
  compact?: boolean;
  tooltip?: string;
}
