// The list items repeat the markup of QuickLinks' item, inside a .map.
export function NavMenu({ items }) {
  return (
    <nav>
      <ul className="menu">
        {items.map((item) => (
          <li key={item.id} className="menu-item" role="none">
            <a href={item.url} title={item.label}>
              <Icon name={item.icon} />
              <span>{item.label}</span>
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}

export function QuickLinks({ home }) {
  return (
    <aside>
      {home && (
        <li key="home" className="quick" role="none">
          <a href={home.url} title="Home">
            <Icon name="home" />
            <span>{home.label}</span>
          </a>
        </li>
      )}
    </aside>
  );
}

function Icon({ name }) {
  return <i className={"icon icon-" + name} />;
}
