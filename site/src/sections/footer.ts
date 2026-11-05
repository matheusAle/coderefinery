/** Footer: status-bar style. Sources left (links), `exit 0` right. Static (§14.5). */
import { fromHTML, html } from "../dom";

export function mountFooter(): HTMLElement {
  return fromHTML(html`
    <footer class="section footer" aria-label="sources">
      <div class="statusbar">
        <span>
          sources:
          <a href="https://www.faros.ai/blog/ai-productivity-paradox-report-2025" rel="noopener">Faros 2025</a> ·
          <a href="https://faros.ai/blog/ai-acceleration-whiplash-takeaways" rel="noopener">Faros 2026</a> ·
          <a href="https://dora.dev/research/2025/dora-report/" rel="noopener">DORA 2025</a> ·
          <a href="https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/" rel="noopener">Sonar 2026</a> ·
          <a href="https://survey.stackoverflow.co/2025/ai" rel="noopener">Stack Overflow 2025</a> ·
          <a href="https://www.techtimes.com/articles/329367/20261022/sequoia-backed-blacksmith-raises-66m-1b-fix-ci-broken-ai-coding-agents.htm" rel="noopener">Blacksmith 2026</a> ·
          <a href="https://arxiv.org/html/2606.18168v1" rel="noopener">arXiv 2606.18168</a> ·
          <a href="https://github.com/anthropics/claude-code/issues/104504" rel="noopener">claude-code #104504</a> ·
          <a href="https://newsletter.kentbeck.com/p/augmented-coding-beyond-the-vibes" rel="noopener">Kent Beck</a>
        </span>
        <span class="exit ingot">exit 0</span>
      </div>
    </footer>
  `);
}
