interface User {
  name: string;
  avatarUrl: string;
}

interface DashboardProps {
  user: User;
  theme: string;
  locale: string;
  notifications: number;
  onLogout: () => void;
}

export function Dashboard({ user, theme, locale, notifications, onLogout }: DashboardProps) {
  return (
    <Layout user={user} theme={theme} locale={locale} notifications={notifications} onLogout={onLogout} />
  );
}

function Layout(props: DashboardProps) {
  return (
    <main className={props.theme}>
      <Sidebar user={props.user} locale={props.locale} notifications={props.notifications} onLogout={props.onLogout} />
    </main>
  );
}

function Sidebar({ user, locale, notifications, onLogout }: Omit<DashboardProps, "theme">) {
  return (
    <aside lang={locale}>
      <img src={user.avatarUrl} alt={user.name} />
      {notifications > 0 && <span>{notifications}</span>}
      <button onClick={onLogout}>Log out</button>
    </aside>
  );
}
