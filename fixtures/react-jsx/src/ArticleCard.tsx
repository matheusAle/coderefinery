type Article = { id: string; title: string; cover: string; summary: string; date: string };

// The card markup again, with one more element in the header:
// a near duplicate.
export function ArticleCard({ article, onRead }: { article: Article; onRead: (id: string) => void }) {
  return (
    <section className="card article">
      <header className="card-header">
        <img src={article.cover} alt="" />
        <h2>{article.title}</h2>
        <time>{article.date}</time>
      </header>
      <p className="card-body">{article.summary}</p>
      <footer>
        <button type="button" onClick={() => onRead(article.id)}>
          Read
        </button>
      </footer>
    </section>
  );
}
