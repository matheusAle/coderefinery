type Team = { id: string; title: string; logo: string; description: string };

export function TeamCard({ team, onJoin }: { team: Team; onJoin: (id: string) => void }) {
  return (
    <section className="card team">
      <header className="team-header">
        <img src={team.logo} alt="team logo" />
        <h2>{team.title}</h2>
      </header>
      <p className="team-body">{team.description}</p>
      <footer>
        <button type="button" onClick={() => onJoin(team.id)}>
          Join the team
        </button>
      </footer>
    </section>
  );
}

// Too small to report, though it repeats Badge's markup exactly.
export function TeamBadge({ name }: { name: string }) {
  return (
    <span className="badge">
      <b>{name}</b>
    </span>
  );
}
