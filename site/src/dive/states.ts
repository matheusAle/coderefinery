/**
 * The four states of the walkthrough (K1–K4, motion-spec §13.3): the agent's
 * session on the left and the outputs pane on the right (`before`, `what the
 * agent changed`, `after`), the heading, the hint and the status bar. Copy is
 * verbatim from designs/reference/page-reference.html. The code itself lives
 * on /math now; the dive shows the CLI and the agent.
 */

export type Tone = "molten" | "slag" | "ingot" | "ash" | "chalk" | "ember";

export type SessionItem =
  /** a typed command (`$ …`) */
  | { kind: "cmd"; text: string }
  /** a line of the gate's output; `exit` lines fire the verdict flash */
  | { kind: "out"; text: string; tone: Tone; exit?: boolean }
  /** the agent reading the output (revealed) */
  | { kind: "agent"; text: string }
  /** a tool call: the header types, the body lines reveal, `done` lands */
  | { kind: "tool"; head: string; body: string[] };

export interface OutRow {
  /** table cells (a grid row) or one free line */
  cells?: string[];
  text?: string;
  tone: Tone;
  /** index of a cell whose number rolls in */
  roll?: number;
  exit?: boolean;
  bold?: boolean;
}

export interface OutBlock {
  label: string;
  head?: string[];
  rows: OutRow[];
}

export interface DiveState {
  key: "crap" | "dry" | "mutate" | "tests";
  n: number;
  title: string;
  hintKicker: string;
  hint: string;
  session: SessionItem[];
  before: OutBlock;
  changed: OutBlock;
  after: OutBlock;
  /** the card's status bar (§0c): the gate's one-line verdict and its exit code */
  status: { verdict: string; exit: string; tone: "slag" | "ingot" | "ash" };
}

const CRAP_HEAD = ["Function", "Namespace", "CC", "Cov%", "CRAP"];
const MUT_HEAD = ["Function", "Killed", "Survived", "Uncovered", "Sites", "Score"];

export const STATES: DiveState[] = [
  {
    key: "crap",
    n: 1,
    title: "The gate speaks first. The agent reads, then edits.",
    hintKicker: "CRAP = CC² × (1 − cov)³ + CC",
    hint: "Nine paths, a fifth covered: `81 × 0.512 + 9 = 50.5`. Ten or more fails. The agent does not argue with a number; it splits.",
    session: [
      { kind: "cmd", text: "coderefinery crap --changed --threshold 9.9999" },
      { kind: "out", text: "billing/pricing.py:price    CRAP 50.5    cov 20.0    cc 9", tone: "molten" },
      { kind: "out", text: "CRAP threshold exceeded: 50.5 > 9.9999 · exit 2", tone: "slag", exit: true },
      { kind: "agent", text: "Nine decisions in one function, one of them covered. Splitting `price()` into `_item_cost` and `_apply_tier`, turning the tier chain into a table, and testing each piece." },
      { kind: "tool", head: "▸ Edit billing/pricing.py", body: ["apply_patch · −12 +9 · price() → price, _item_cost, _apply_tier, TIER_RATE"] },
      { kind: "tool", head: "▸ Write tests/test_pricing.py", body: ["+4 cases · digital, physical, each tier, coupon"] },
      { kind: "cmd", text: "coderefinery crap --changed --threshold 9.9999" },
      { kind: "out", text: "price 3 · _item_cost 3 · _apply_tier 1 · all 100 % covered", tone: "ingot" },
      { kind: "out", text: "0 functions above the gate · exit 0", tone: "ingot", exit: true },
    ],
    before: {
      label: "before",
      head: CRAP_HEAD,
      rows: [
        { cells: ["price", "billing", "9", "20.0", "50.5"], tone: "molten", roll: 4 },
        { text: "exit 2", tone: "slag", exit: true, bold: true },
      ],
    },
    changed: {
      label: "what the agent changed",
      rows: [
        { text: "− for item in order.items: … (lines 3–7)", tone: "ash" },
        { text: "+ total = sum(_item_cost(i) for i in order.items)", tone: "ingot" },
        { text: "− if user.tier == 'gold': … (lines 8–11)", tone: "ash" },
        { text: "+ total = _apply_tier(total, user.tier)", tone: "ingot" },
        { text: "+ def _item_cost(item) · + def _apply_tier(total, tier)", tone: "ingot" },
        { text: "+ tests: 4 cases · cov 20 % → 100 %", tone: "ingot" },
        { text: "−12 +9 · 2 files", tone: "ash", bold: true },
      ],
    },
    after: {
      label: "after",
      head: CRAP_HEAD,
      rows: [
        { cells: ["price", "billing", "3", "100.0", "3.0"], tone: "ingot" },
        { cells: ["_item_cost", "billing", "3", "100.0", "3.0"], tone: "ingot" },
        { cells: ["_apply_tier", "billing", "1", "100.0", "1.0"], tone: "ingot" },
        { text: "exit 0", tone: "ingot", exit: true, bold: true },
      ],
    },
    status: { verdict: "crap · 50.5 → 3 · the agent split, the gate agreed", exit: "exit 2 → exit 0", tone: "ingot" },
  },
  {
    key: "dry",
    n: 2,
    title: "Same body, new names. The agent merges.",
    hintKicker: "score = |A ∩ B| / |A ∪ B|",
    hint: "Fingerprints of the syntax tree with names erased: `20 of 22` shared, `0.91`. At `0.8` or above, one body. The report says so; the skill reads the report.",
    session: [
      { kind: "cmd", text: "coderefinery dry --changed --threshold 0.8" },
      { kind: "out", text: "DUPLICATE score=0.91", tone: "molten" },
      { kind: "out", text: "  billing/pricing.py:1-6", tone: "molten" },
      { kind: "out", text: "  billing/quotes.py:8-13", tone: "molten" },
      { kind: "out", text: "1 pair at or above 0.80 · exit 0 · verdict is in the report", tone: "ash", exit: true },
      { kind: "agent", text: "`quote()` is `price()` with the arguments renamed. Both delegate to one `_settle(items, tier, discount)`; the public names stay so nothing that calls them changes." },
      { kind: "tool", head: "▸ Edit billing/pricing.py · billing/quotes.py", body: ["apply_patch · −10 +4 · +_settle(items, tier, discount=None) · price and quote delegate"] },
      { kind: "cmd", text: "coderefinery dry --changed --threshold 0.8" },
      { kind: "out", text: "No duplicate candidates found. · exit 0", tone: "ingot", exit: true },
    ],
    before: {
      label: "before",
      rows: [
        { text: "DUPLICATE score=0.91", tone: "molten" },
        { text: "billing/pricing.py:1-6", tone: "chalk" },
        { text: "billing/quotes.py:8-13", tone: "chalk" },
        { text: "1 pair · report", tone: "ember", bold: true },
      ],
    },
    changed: {
      label: "what the agent changed",
      rows: [
        { text: "+ def _settle(items, tier, discount=None): … one body", tone: "ingot" },
        { text: "− price(): 5-line body", tone: "ash" },
        { text: "+ return _settle(order.items, user.tier, coupon)", tone: "ingot" },
        { text: "− quote(): 5-line body", tone: "ash" },
        { text: "+ return _settle(cart.items, account.tier, promo)", tone: "ingot" },
        { text: "−10 +4 · 2 files", tone: "ash", bold: true },
      ],
    },
    after: {
      label: "after",
      rows: [
        { text: "No duplicate candidates found.", tone: "ingot" },
        { text: "exit 0", tone: "ingot", exit: true, bold: true },
      ],
    },
    status: { verdict: "dry · 0.91 → none · one implementation, two names", exit: "report → clean", tone: "ingot" },
  },
  {
    key: "mutate",
    n: 3,
    title: "Two survive. The agent writes the tests that kill them.",
    hintKicker: "would the tests notice?",
    hint: "One edit per mutant, the suite per edit. A survivor is a bug the suite would ship. One survivor fails; the agent adds the assertion, never weakens the test.",
    session: [
      { kind: "cmd", text: "coderefinery mutate --changed --use-existing-coverage" },
      { kind: "out", text: "12 sites · 12 mutants · baseline 1.8 s · timeout 18 s", tone: "ash" },
      { kind: "out", text: "SURVIVED  billing/pricing.py:6   0 → 1", tone: "molten" },
      { kind: "out", text: "SURVIVED  billing/pricing.py:9   1 → 0", tone: "molten" },
      { kind: "out", text: "killed 10 · survived 2 · exit 3", tone: "slag", exit: true },
      { kind: "agent", text: "The floor at `max(total, 0)` and the default tier rate are never asserted. A coupon larger than the total must price to exactly `0`; an unknown tier must pay full price. Adding both cases, not touching the code." },
      {
        kind: "tool",
        head: "▸ Write tests/test_pricing.py",
        body: [
          "+def test_coupon_never_goes_negative(): assert price(order(5), gold, coupon(50)) == 0.0",
          "+def test_unknown_tier_pays_full_price(): assert price(order(100), bronze) == 100.0",
        ],
      },
      { kind: "cmd", text: "coderefinery mutate --changed --use-existing-coverage" },
      { kind: "out", text: "killed 12 · survived 0 · re-ran 2 mutants only · exit 0", tone: "ingot", exit: true },
    ],
    before: {
      label: "before",
      head: MUT_HEAD,
      rows: [
        { cells: ["_settle", "5", "1", "0", "6", "83.3%"], tone: "molten" },
        { cells: ["_apply_tier", "2", "1", "0", "3", "66.7%"], tone: "molten" },
        { cells: ["_item_cost", "3", "0", "0", "3", "100.0%"], tone: "ash" },
        { text: "exit 3", tone: "slag", exit: true, bold: true },
      ],
    },
    changed: {
      label: "what the agent changed",
      rows: [
        { text: "+ test_coupon_never_goes_negative   kills 0 → 1", tone: "ingot" },
        { text: "+ test_unknown_tier_pays_full_price   kills 1 → 0", tone: "ingot" },
        { text: "no source line changed", tone: "ash" },
        { text: "+2 tests · 1 file", tone: "ash", bold: true },
      ],
    },
    after: {
      label: "after",
      rows: [
        { cells: ["_settle", "6", "0", "0", "6", "100.0%"], tone: "ingot" },
        { cells: ["_apply_tier", "3", "0", "0", "3", "100.0%"], tone: "ingot" },
        { cells: ["_item_cost", "3", "0", "0", "3", "100.0%"], tone: "ingot" },
        { text: "exit 0", tone: "ingot", exit: true, bold: true },
      ],
    },
    status: { verdict: "mutate · survived 2 → 0 · two assertions that can fail", exit: "exit 3 → exit 0", tone: "ingot" },
  },
  {
    key: "tests",
    n: 4,
    title: "Two tests catch nothing. Deleted.",
    hintKicker: "relevance = mutants this test alone kills",
    hint: "Zero kills: it would pass with every function returning nothing. Shadowed: other tests already catch everything it does. Copies drift; one parametrized case does not.",
    session: [
      { kind: "cmd", text: "coderefinery tests --changed" },
      { kind: "out", text: "test                               kills  unique  shadowed by", tone: "ash" },
      { kind: "out", text: "test_tier_rate[gold]                   4       2  —", tone: "ingot" },
      { kind: "out", text: "test_coupon_never_goes_negative        1       1  —", tone: "ingot" },
      { kind: "out", text: "test_price_returns_a_number            0       0  —", tone: "molten" },
      { kind: "out", text: "test_settle_called                     0       0  —", tone: "molten" },
      { kind: "out", text: "2 tests with no kills · exit 3", tone: "slag", exit: true },
      { kind: "cmd", text: "coderefinery dry tests/test_pricing.py" },
      { kind: "out", text: "DUPLICATE score=0.93 ×2 · test_gold_discount ↔ test_silver_discount, test_bronze_pays_full", tone: "molten" },
      { kind: "agent", text: "`isinstance()` passes for any float; the spy passes whenever `_settle` is called. Neither can fail on a wrong price. Deleting both. The three tier tests are one test with three rows: parametrizing." },
      { kind: "tool", head: "▸ Edit tests/test_pricing.py", body: ["apply_patch · −14 +5 · −test_price_returns_a_number · −test_settle_called · +@pytest.mark.parametrize('tier,expected', …) test_tier_rate"] },
      { kind: "cmd", text: "coderefinery tests --changed" },
      { kind: "out", text: "5 tests · 12 kills · 0 with zero kills · 0 shadowed · exit 0", tone: "ingot", exit: true },
    ],
    before: {
      label: "before",
      head: ["test", "kills", "unique"],
      rows: [
        { cells: ["test_price_returns_a_number", "0", "0"], tone: "molten" },
        { cells: ["test_settle_called", "0", "0"], tone: "molten" },
        { text: "test_gold/silver/bronze   3 copies at 0.93", tone: "ash" },
        { text: "exit 3", tone: "slag", exit: true, bold: true },
      ],
    },
    changed: {
      label: "what the agent changed",
      rows: [
        { text: "− test_price_returns_a_number", tone: "slag" },
        { text: "− test_settle_called", tone: "slag" },
        { text: "− test_gold_discount · test_silver_discount · test_bronze_pays_full", tone: "slag" },
        { text: "+ @parametrize test_tier_rate[gold | silver | bronze]", tone: "ingot" },
        { text: "−14 +5 · 1 file", tone: "ash", bold: true },
      ],
    },
    after: {
      label: "after",
      rows: [
        { text: "5 tests · 12 kills · every test kills something only it kills", tone: "ingot" },
        { text: "No duplicate candidates found.", tone: "ingot" },
        { text: "exit 0", tone: "ingot", exit: true, bold: true },
      ],
    },
    status: { verdict: "tests · 2 deleted, 3 merged, 0 kills lost", exit: "exit 3 → exit 0", tone: "ingot" },
  },
];
