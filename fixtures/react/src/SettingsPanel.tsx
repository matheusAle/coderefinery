type SettingsPanelProps = {
  title: string;
  subtitle?: string;
  darkMode: boolean;
  compact: boolean;
  showHeader?: boolean;
  showFooter?: boolean;
  disabled?: boolean;
  onSave: () => void;
  onCancel?: () => void;
};

export const SettingsPanel = ({
  title,
  subtitle,
  darkMode,
  compact,
  showHeader = true,
  showFooter = false,
  disabled = false,
  onSave,
  onCancel,
}: SettingsPanelProps) => (
  <section className={darkMode ? "dark" : "light"} data-compact={compact}>
    {showHeader && <h2>{title}</h2>}
    {subtitle && <p>{subtitle}</p>}
    <button disabled={disabled} onClick={onSave}>
      Save
    </button>
    {showFooter && onCancel && <button onClick={onCancel}>Cancel</button>}
  </section>
);
