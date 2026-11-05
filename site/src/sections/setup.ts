/**
 * §08 · recommended setup, the last card before the footer (§12): the kicker
 * fades and the headline types, the hook file and the session pane reveal,
 * then the transcript replays inside the session pane's slot (refusal, the
 * agent writing two tests with the tool call typing, second commit, hook
 * passes, `exit 0`, the commit hash in ingot), the status bar fades last.
 * `id="hook"` is the target of `/#hook` from the agents page.
 */
import { esc, fromHTML, html, q, qa } from "../dom";
import { typer } from "../motion/typer";
import { onMount } from "../motion/mount";
import { reducedMotion } from "../motion/scroll";
import { head, statusbar } from "./common";

const SETTINGS = `{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash(git commit*)",
      "hooks": [{
        "type": "command",
        "command": "coderefinery crap --changed --threshold 9.9999 && coderefinery dry --changed && coderefinery mutate --changed --use-existing-coverage"
      }]
    }]
  }
}`;

/**
 * The settings file, one block per line with a hanging indent: a line that wraps continues at the column where its
 * string value starts (the long "command" line), or at its own indent.
 */
function hookLines(): string {
  return SETTINGS.split("\n")
    .map((line) => {
      const indent = line.length - line.trimStart().length;
      const value = line.indexOf(': "');
      const hang = value >= 0 ? value + 3 : indent;
      return `<span class="jl" style="--hang:${hang}ch">${esc(line) || " "}</span>`;
    })
    .join("");
}

export function mountSetup(): HTMLElement {
  const el = fromHTML(html`
    <section class="section setup type-step" id="hook" aria-labelledby="setup-h">
      ${head("§08 · recommended setup · two minutes", "Add the hook. Install the skill.<br />Let the agent police itself.", "setup-h")}
      <div class="setup-panes">
        <figure class="hook pane box" data-slot="card" aria-label="hook configuration">
          <div class="pane-bar"><span>.claude/settings.json · Cursor: .cursor/hooks.json</span></div>
          <pre class="hook-json mono">${hookLines()}</pre>
        </figure>
        <figure class="session pane box setup-session" data-slot="card" aria-label="The same session, continued: the hook refuses, the agent writes tests, the commit passes">
          <div class="pane-bar"><span>claude code · ~/work/billing</span><span class="ingot">● live</span></div>
          <div class="transcript">
            <div class="cmd sline" data-at="0.04"><span class="prompt molten">$</span><span>git commit -m "add quote endpoint"</span></div>
            <div class="tool box sline" data-at="0.10">
              <div class="tool-h">▸ PreToolUse hook · coderefinery mutate --changed</div>
              <div class="tool-b">
                <span class="sline" data-at="0.14">SURVIVED&nbsp;&nbsp;billing/pricing.py:6&nbsp;&nbsp;&nbsp;0 → 1</span>
                <span class="sline" data-at="0.17">SURVIVED&nbsp;&nbsp;billing/pricing.py:9&nbsp;&nbsp;&nbsp;1 → 0</span>
                <span class="sline slag" data-at="0.20">exit 3</span>
              </div>
            </div>
            <div class="verdict slag sline" data-at="0.25">✗ Bash(git commit) blocked by hook</div>
            <div class="agent box sline" data-at="0.30"><span class="agent-k">agent</span><p>Two survivors. The tests never assert the floor or an unknown tier. I will add tests that fail on these mutants, not weaken the ones that exist.</p></div>
            <div class="tool box sline" data-at="0.38">
              <div class="tool-h">▸ Write tests/test_pricing.py</div>
              <div class="tool-b">
                <span data-typer="+def test_coupon_never_goes_negative(): …" data-from="0.40" data-to="0.50"></span>
                <span data-typer="+def test_unknown_tier_pays_full_price(): …" data-from="0.50" data-to="0.60"></span>
              </div>
            </div>
            <div class="cmd sline" data-at="0.64"><span class="prompt molten">$</span><span>git commit -m "add quote endpoint"</span></div>
            <!-- the passing hook run as one line (was a tool box with two body lines): the card fits with the §0c floors at 100 vh minus the footer -->
            <div class="verdict ingot sline" data-at="0.70">▸ PreToolUse hook · coderefinery mutate --changed · killed 12  survived 0 · exit 0</div>
            <div class="verdict ingot sline" data-at="0.90">✓ [main 4f2c1d9] add quote endpoint · 3 files changed</div>
          </div>
        </figure>
        <ul class="hook-notes mono box" data-slot="card" aria-label="the three steps">
            <li>· step 1 · <code class="ic">npx skills add coderefinery</code> installs the skill: gate order, thresholds, what to do per verdict</li>
            <li>· step 2 · the hook above: <code class="ic">exit ≠ 0</code> blocks the tool call; the agent sees stderr and fixes the code</li>
            <li>· step 3 · the same three commands in CI: same exit codes, same <code class="ic">.metrics/</code>, no second config</li>
            <li>· what the skill adds: TDD first, split before test, delete what no honest test can kill</li>
        </ul>
      </div>
      ${statusbar("what you get: one refused commit, two new tests, zero deleted, no human in the loop until the gate is green", 0, { dot: true })}
    </section>
  `);

  const lines = qa(el, ".sline");
  const typers = qa(el, "[data-typer]").map((h) => {
    const t = typer(h.dataset.typer!);
    h.appendChild(t.el);
    return { t, from: Number(h.dataset.from), to: Number(h.dataset.to), host: h };
  });
  const session = q(el, ".setup-session");
  const rm = reducedMotion();

  // this card is unpinned (§0c): the transcript is the session block's text, revealed by its top-down clip with the
  // block's words, not scrubbed as a walkthrough
  onMount(el, (_q, _slot, text) => {
    const on = rm || text(session) > 0;
    lines.forEach((l) => l.classList.toggle("in", on));
    typers.forEach(({ t, host }) => {
      host.classList.toggle("in", on);
      t.set(on ? 1 : 0);
      t.cursor(false);
    });
  });

  return el;
}
