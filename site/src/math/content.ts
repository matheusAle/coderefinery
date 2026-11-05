/**
 * /math explainer content (§16), copied verbatim from designs/reference/math-reference.html:
 * formula per state (molten = the newest substitution), the code each gate evaluates, the per-state code
 * annotations (§16.2, in the order the card describes), card copy (backticks mark inline code, §0b) and the
 * status bar. Generated once from the reference by .tools/mathdata.py and edited by hand where noted.
 */

export type Tone = "molten" | "ingot" | "ember" | "slag";
export interface Term {
  text: string;
  molten?: boolean;
}
export interface Annotation {
  /** 1-based line number */
  line: number;
  tint?: Tone;
  note?: string;
  tone?: Tone;
  /** annotations with the same step land together (the dry line pairs) */
  step?: number;
}
export type Visual = "squares" | "coverage" | "markers";
export interface MathState {
  title: string;
  body: string;
  rule: string;
  formula: Term[];
  annotations: Annotation[];
  visual?: Visual;
  status: string;
}
export interface Gate {
  id: "crap" | "dry" | "mutate" | "tests";
  kicker: string;
  file: string;
  code: string[];
  exit: { code: string; tone: "slag" | "ingot" };
  /** the formula as written before the first substitution (shown until state 1 starts computing) */
  pre?: Term[];
  states: MathState[];
}

export const CRAP: Gate = {
  id: "crap",
  kicker: "§01 · crap",
  file: "billing/pricing.py",
  exit: { code: "exit 2", tone: "slag" },
  pre: [{ text: "CRAP = CC² × (1 − cov)³ + CC" }],
  code: [
    "def price(order, user, coupon=None):",
    "    total = 0",
    "    for item in order.items:",
    "        if item.kind == 'digital':",
    "            total += item.price",
    "        elif item.kind == 'physical':",
    "            total += item.price + item.weight * 0.5",
    "    if user.tier == 'gold':",
    "        total *= 0.9",
    "    elif user.tier == 'silver':",
    "        total *= 0.95",
    "    if coupon and coupon.valid():",
    "        total -= coupon.amount",
    "    if total <= 0:",
    "        total = 0",
    "    return round(total, 2)",
  ],
  states: [
    {
      title: "Count the decisions.",
      body: "Every if, loop, case and boolean operator in `price()` is a path. `CC` is one plus the number of them. Eight decisions here, so `CC = 9`. The square is the point: paths multiply.",
      rule: "`CC 9` means nine inputs to walk every path. One test walks one.",
      formula: [{ text: "CRAP =" }, { text: "9²", molten: true }, { text: "× (1 − cov)³ +" }, { text: "9" }],
      annotations: [
        { line: 3, tint: "molten", note: "1 · for", tone: "molten" },
        { line: 4, tint: "molten", note: "2 · if", tone: "molten" },
        { line: 6, tint: "molten", note: "3 · elif", tone: "molten" },
        { line: 8, tint: "molten", note: "4 · if", tone: "molten" },
        { line: 10, tint: "molten", note: "5 · elif", tone: "molten" },
        { line: 12, tint: "molten", note: "6 · if · 7 · and", tone: "molten" },
        { line: 14, tint: "molten", note: "8 · if", tone: "molten" },
      ],
      visual: "squares",
      status: "crap · count the decisions",
    },
    {
      title: "Discount by what the tests never walk.",
      body: "`cov` is the share of lines a test executed. The one test covers the digital path: `3 of 15` lines, `cov = 0.20`. The cube keeps `0.8³ = 0.512` of the risk; at `80 %` coverage only `0.008` would remain.",
      rule: "Below `50 %` coverage the score barely moves. Test the branches, not the percentage.",
      formula: [{ text: "CRAP = 9² ×" }, { text: "(1 − 0.20)³", molten: true }, { text: "+ 9" }],
      annotations: [
        { line: 1, tint: "ingot", note: "covered", tone: "ingot" },
        { line: 2, tint: "ingot", note: "covered", tone: "ingot" },
        { line: 3, tint: "ingot", note: "covered", tone: "ingot" },
        { line: 4, tint: "ingot", note: "covered", tone: "ingot" },
        { line: 5, tint: "ingot", note: "covered", tone: "ingot" },
        { line: 6, tint: "ember", note: "never run", tone: "ember" },
        { line: 7, tint: "ember", note: "never run", tone: "ember" },
        { line: 8, tint: "ember", note: "never run", tone: "ember" },
        { line: 10, tint: "ember", note: "never run", tone: "ember" },
        { line: 12, tint: "ember", note: "never run", tone: "ember" },
        { line: 14, tint: "ember", note: "never run", tone: "ember" },
      ],
      visual: "coverage",
      status: "crap · discount by what the tests never walk",
    },
    {
      title: "Compare with ten.",
      body: "`81 × 0.512 + 9 = 50.47`. Ten is the line, and the skill fails at exactly 10 by passing `--threshold 9.9999`. Savoia drew it at 30 in 2007; ub-mode draws it at 10 so a function still reads in one sitting.",
      rule: "`CRAP ≥ 10 → exit 2`. At `CC 10` or more no coverage can pass: split first.",
      formula: [{ text: "CRAP = 81 × 0.512 + 9 =" }, { text: "50.5", molten: true }],
      annotations: [
        { line: 1, tint: "molten", note: "CRAP 50.5 · exit 2", tone: "molten", step: 0 },
        { line: 2, tint: "molten", step: 0 },
        { line: 3, tint: "molten", step: 0 },
        { line: 4, tint: "molten", step: 0 },
        { line: 5, tint: "molten", step: 0 },
        { line: 6, tint: "molten", step: 0 },
        { line: 7, tint: "molten", step: 0 },
        { line: 8, tint: "molten", step: 0 },
        { line: 9, tint: "molten", step: 0 },
        { line: 10, tint: "molten", step: 0 },
        { line: 11, tint: "molten", step: 0 },
        { line: 12, tint: "molten", step: 0 },
        { line: 13, tint: "molten", step: 0 },
        { line: 14, tint: "molten", step: 0 },
        { line: 15, tint: "molten", step: 0 },
        { line: 16, tint: "molten", step: 0 },
      ],
      visual: "markers",
      status: "crap · compare with ten",
    },
  ],
};

export const DRY: Gate = {
  id: "dry",
  kicker: "§02 · dry",
  file: "billing/pricing.py · billing/quotes.py",
  exit: { code: "exit 0", tone: "ingot" },
  code: [
    "def price(order, user, coupon=None):",
    "    total = sum(_item_cost(i) for i in order.items)",
    "    total = _apply_tier(total, user.tier)",
    "    if coupon and coupon.valid():",
    "        total -= coupon.amount",
    "    return round(max(total, 0), 2)",
    "",
    "def quote(cart, account, promo=None):",
    "    amount = sum(_item_cost(i) for i in cart.items)",
    "    amount = _apply_tier(amount, account.tier)",
    "    if promo and promo.valid():",
    "        amount -= promo.amount",
    "    return round(max(amount, 0), 2)",
  ],
  states: [
    {
      title: "Erase the names. Keep the shape.",
      body: "Each function becomes a tree of calls, operators and keywords; local names and literals become placeholders. Every subtree, printed, is one fingerprint. `price()` has 22, `quote()` has 22, and renaming `order` to `cart` changes none of them.",
      rule: "Rename every variable: `A` is unchanged. Replace `sum` with `max`: five fingerprints change.",
      formula: [{ text: "A = fp(price) ·" }, { text: "22", molten: true }, { text: "B = fp(quote) ·" }, { text: "22" }],
      annotations: [
        { line: 2, tint: "ember", note: "order → :symbol", tone: "ember" },
        { line: 3, tint: "ember", note: "user → :symbol", tone: "ember" },
        { line: 9, tint: "ember", note: "cart → :symbol", tone: "ember" },
        { line: 10, tint: "ember", note: "account → :symbol", tone: "ember" },
      ],
      status: "dry · erase the names. keep the shape",
    },
    {
      title: "Measure the overlap.",
      body: "Jaccard: the fingerprints both share, over all the fingerprints either has. The two bodies share `20 of 22`. The two that differ are the two `def` headers, whose names are kept in head position.",
      rule: "Shape, not text: a search for the copied line would find nothing.",
      formula: [{ text: "score =" }, { text: "20", molten: true }, { text: "/" }, { text: "22" }],
      annotations: [
        { line: 2, tint: "ingot", note: "= line 9", tone: "ingot", step: 0 },
        { line: 3, tint: "ingot", note: "= line 10", tone: "ingot", step: 1 },
        { line: 4, tint: "ingot", note: "= line 11", tone: "ingot", step: 2 },
        { line: 5, tint: "ingot", note: "= line 12", tone: "ingot", step: 3 },
        { line: 6, tint: "ingot", note: "= line 13", tone: "ingot", step: 4 },
        { line: 9, tint: "ingot", step: 0 },
        { line: 10, tint: "ingot", step: 1 },
        { line: 11, tint: "ingot", step: 2 },
        { line: 12, tint: "ingot", step: 3 },
        { line: 13, tint: "ingot", step: 4 },
      ],
      status: "dry · measure the overlap",
    },
    {
      title: "Decide at 0.8.",
      body: "The tool's default is `0.82`; the skill lowers it to `0.8`, and a pair on the line is reported. Functions under 20 normalised nodes are never compared. `dry` exits `0` either way; the verdict is the pair in the report, and the skill reads it.",
      rule: "`score ≥ 0.8` → one body, two names. Merge it.",
      formula: [{ text: "score = 0.91 ≥ 0.80 →" }, { text: "DUPLICATE", molten: true }],
      annotations: [
        { line: 1, tint: "molten", step: 0, note: "DUPLICATE score=0.91", tone: "molten" },
        { line: 2, tint: "molten", step: 0 },
        { line: 3, tint: "molten", step: 0 },
        { line: 4, tint: "molten", step: 0 },
        { line: 5, tint: "molten", step: 0 },
        { line: 6, tint: "molten", step: 0 },
        { line: 8, tint: "molten", step: 0, note: "↔ pricing.py:1-6", tone: "molten" },
        { line: 9, tint: "molten", step: 0 },
        { line: 10, tint: "molten", step: 0 },
        { line: 11, tint: "molten", step: 0 },
        { line: 12, tint: "molten", step: 0 },
        { line: 13, tint: "molten", step: 0 },
      ],
      status: "dry · decide at 0.8",
    },
  ],
};

export const MUTATE: Gate = {
  id: "mutate",
  kicker: "§03 · mutate",
  file: "billing/pricing.py",
  exit: { code: "exit 3", tone: "slag" },
  code: [
    "def _settle(items, tier, discount=None):",
    "    total = sum(_item_cost(i) for i in items)",
    "    total = _apply_tier(total, tier)",
    "    if discount and discount.valid():",
    "        total -= discount.amount",
    "    return round(max(total, 0), 2)",
    "",
    "def _apply_tier(total, tier):",
    "    return total * TIER_RATE.get(tier, 1)",
    "",
    "def test_gold_discount():",
    "    assert price(order(100), gold) == 90.0",
    "def test_coupon():",
    "    assert price(order(100), gold, coupon(10)) == 80.0",
  ],
  states: [
    {
      title: "Change one token.",
      body: "A site is a token the tool knows how to flip; a mutant is the file with one site changed. Twelve sites in these two functions: `and`, `-`, `0`, `*`, `1`, the comparisons. Each mutant is the smallest bug that can be written.",
      rule: "Uncovered sites never run; no test reaches them, so the answer is already known.",
      formula: [{ text: "sites =" }, { text: "12", molten: true }, { text: "mutants =" }, { text: "12" }],
      annotations: [
        { line: 4, tint: "molten", note: "and → or", tone: "molten" },
        { line: 5, tint: "molten", note: "− → +", tone: "molten" },
        { line: 6, tint: "molten", note: "0 → 1", tone: "molten" },
        { line: 9, tint: "molten", note: "* → / · 1 → 0", tone: "molten" },
      ],
      status: "mutate · change one token",
    },
    {
      title: "Run the suite twelve times.",
      body: "The suite runs once per mutant. Red means killed: a test noticed. Green means survived: that line was never really tested. Two survived: the floor at `max(total, 0)` and the default `TIER_RATE.get(tier, 1)`, because no test crosses zero or uses an unknown tier.",
      rule: "No middle ground. A survivor is a specific bug, with a file and a line.",
      formula: [{ text: "score =" }, { text: "10", molten: true }, { text: "/ (" }, { text: "10 + 2" }, { text: ")" }],
      annotations: [
        { line: 4, tint: "ingot", note: "killed", tone: "ingot" },
        { line: 5, tint: "ingot", note: "killed", tone: "ingot" },
        { line: 6, tint: "slag", note: "0 → 1 · SURVIVED", tone: "slag" },
        { line: 9, tint: "slag", note: "1 → 0 · SURVIVED", tone: "slag" },
        { line: 12, note: "kills and→or, −→+", tone: "molten" },
        { line: 14, note: "kills nothing at the floor", tone: "molten" },
      ],
      status: "mutate · run the suite twelve times",
    },
    {
      title: "One survivor fails.",
      body: "`83 %` is not the verdict; `survived 2` is. Any survivor is `exit 3`. The next run re-executes only survivors and functions whose text changed, so the loop is seconds.",
      rule: "`survived > 0 → exit 3`. Add the assertion that kills it, or delete code no honest test can tell apart.",
      formula: [{ text: "score = 10 / 12 =" }, { text: "0.83" }, { text: "· survived" }, { text: "2", molten: true }, { text: "→ exit 3" }],
      annotations: [
        { line: 6, tint: "slag", note: "SURVIVED · needs: a case where the result is exactly 0", tone: "slag" },
        { line: 9, tint: "slag", note: "SURVIVED · needs: an unknown tier, asserted on the result", tone: "slag" },
      ],
      status: "mutate · one survivor fails",
    },
  ],
};

export const TESTS: Gate = {
  id: "tests",
  kicker: "§04 · tests",
  file: "tests/test_pricing.py",
  exit: { code: "exit 3", tone: "slag" },
  code: [
    "def test_tier_rate(tier, expected):",
    "    assert price(order(100), user(tier)) == expected",
    "",
    "def test_coupon_never_goes_negative():",
    "    assert price(order(5), gold, coupon(50)) == 0.0",
    "",
    "def test_quote_gold():",
    "    assert client.post('/quote', gold_cart).json()['total'] == 90.0",
    "",
    "def test_price_returns_a_number():",
    "    assert isinstance(price(order(100), gold), float)",
    "",
    "def test_settle_called(mocker):",
    "    spy = mocker.spy(pricing, '_settle'); price(order(100), gold); assert spy.called",
  ],
  states: [
    {
      title: "Which test killed it?",
      body: "With attribution, every killed mutant records the tests that failed on it. `kills` per test: `4, 1, 5, 0, 0`. A test with zero kills would pass with every function returning nothing.",
      rule: "`kills 0` → delete. It is not a test. (preview: ships with the tests gate)",
      formula: [{ text: "kills(t) = { m : t fails on m } →" }, { text: "4 · 1 · 5 · 0 · 0", molten: true }],
      annotations: [
        { line: 1, tint: "ingot", note: "kills 4", tone: "ingot" },
        { line: 4, tint: "ingot", note: "kills 1", tone: "ingot" },
        { line: 7, tint: "ingot", note: "kills 5", tone: "ingot" },
        { line: 10, tint: "slag", note: "kills 0", tone: "slag" },
        { line: 13, tint: "slag", note: "kills 0 · spies on a private call", tone: "slag" },
      ],
      status: "tests · which test killed it?",
    },
    {
      title: "Which test is the only one?",
      body: "Subtract what the other tests also kill. `test_quote_gold` `kills 5` but none uniquely: `test_tier_rate` and `test_coupon_never_goes_negative` already catch all five. It is fully shadowed.",
      rule: "`unique 0` → merge or delete, unless it guards a seam at a higher level. The report names the shadow.",
      formula: [{ text: "unique(t) = kills(t) − ⋃ kills(others) →" }, { text: "2 · 1 · 0 · 0 · 0", molten: true }],
      annotations: [
        { line: 1, tint: "ingot", note: "unique 2", tone: "ingot" },
        { line: 4, tint: "ingot", note: "unique 1", tone: "ingot" },
        { line: 7, tint: "ember", note: "unique 0 · shadowed by lines 1, 4", tone: "ember" },
        { line: 10, tint: "slag", note: "unique 0", tone: "slag" },
        { line: 13, tint: "slag", note: "unique 0", tone: "slag" },
      ],
      status: "tests · which test is the only one?",
    },
  ],
};

export const GATES: Gate[] = [CRAP, DRY, MUTATE, TESTS];
