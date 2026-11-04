// Different markup: no pair with the cards.
export function Table({ rows }: { rows: { id: string; cells: string[] }[] }) {
  return (
    <table>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id}>
            {r.cells.map((c, i) => (
              <td key={i}>{c}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
