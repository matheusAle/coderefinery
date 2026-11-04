type User = { id: string; name: string; avatar: string; bio: string };

// Same markup as TeamCard: only text, class names and values differ.
export function ProfileCard({ user, onFollow }: { user: User; onFollow: (id: string) => void }) {
  return (
    <section className="card profile">
      <header className="card-header">
        <img src={user.avatar} alt="" />
        <h2>{user.name}</h2>
      </header>
      <p className="card-body">{user.bio}</p>
      <footer>
        <button type="button" onClick={() => onFollow(user.id)}>
          Follow
        </button>
      </footer>
    </section>
  );
}
