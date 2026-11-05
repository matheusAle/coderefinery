# Developer pains with AI code quality: reviewing agent-written code (2025–2026)

Scope: real-world complaints from developers and engineering leads about reviewing code written by AI coding agents (Claude Code, Cursor, Copilot, Codex, Devin, Windsurf). Excludes AI-generated tests and mutation testing (covered elsewhere). Dates: 2025–2026 unless flagged as a foundational 2024 source. Where I could only reach secondary coverage, that is stated.

Source-quality legend used throughout: [primary] = original post/report; [vendor] = company that sells review/analytics tooling (directionally useful, incentive to overstate); [secondary] = press/blog summarising someone else's data.

---

## Key question 1: What specific failure modes of agent-written code do experienced developers report most often?

### Takeaway
The dominant complaint is not that agent code is obviously bad, but that it is *believably* wrong: syntactically clean, stylistically slightly foreign, over-built, repetitive, and carrying subtle logic/edge-case bugs that reviewers skim past. Secondary but frequent complaints are duplicated helpers instead of reuse, no unifying design intent, bloat/over-engineering, security weaknesses, and authors who cannot explain their own PR.

### Cited Findings

**"Looks right but isn't" / subtle bugs**
- HN "Ask HN: Is anyone else sick of AI splattered code" (Sept 17 2025, 89 points, 84 comments). User juancn: "the code is believable crap, which requires mindbogglingly boring and intense code reviews." User throwacct: "I spent 2 days trying to find the reason the code wasn't working the way it supposed to work." User elviejo: "With llms we know for a fact that if the code matches reality, or expectations, it's a happy accident." User jcranmer: "AI makes mistakes in very different ways from the way humans make mistakes." — [HN thread, primary](https://news.ycombinator.com/item?id=45278819)
- HN "AI coding agents generate more code, but not more software" (365 points, 314 comments, ~Oct 2026, linked to an Ars Technica article). User dathinab: "large blobs of not well written AI generated code which produces a slew of subtle easy to overlook bugs." User duttish: "Whenever I've tried skipping code review I've later found janky crap or weird decisions made." User rbongers: "AI is great at reviewing code. It is not great at reviewing intention." — [HN thread, primary](https://news.ycombinator.com/item?id=50059523)
- Greptile (vendor, AI review tool; undated page referencing Q3 2026 data): "The failure mode isn't sloppiness. It's confidence." / "The code looks right, which means reviewers are more likely to skim it." / typical bug is "a function that handles 95% of cases perfectly but silently drops the other 5%" and "the bugs tend to be cross-file logic issues." — [Greptile, vendor](https://greptile.com/blog/ai-code-review)
- Bryan Finster (continuous-delivery practitioner, Substack, Mar 4 2026): AI code "often looks right" but lacks organizational conventions, tribal knowledge, and integration quirks. — [Finster, primary](https://bryanfinster.substack.com/p/ai-broke-your-code-review-heres-how)
- Sonar 2026 State of Code survey (1,100+ developers, press release Jan 8 2026): 88% cite negative AI impacts on technical debt, "including 53% who say AI generates code that looks correct but isn't reliable." — [Sonar press release, vendor/primary survey](https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/)
- Stack Overflow Developer Survey 2025 (secondary summaries): 66% cited "AI solutions that are almost right, but not quite" as their top frustration; 45.2% said debugging AI-generated code is more time-consuming. — [RDEL summary, secondary](https://rdel.substack.com/p/rdel-105-top-findings-from-the-2025); [TechRepublic, secondary](https://www.techrepublic.com/article/news-developer-trust-in-ai-declines/)
- CodeRabbit "State of AI vs Human Code Generation" (vendor, Dec 17 2025; 470 open-source PRs: 320 AI-co-authored, 150 human-only; authorship inferred from co-author signals, acknowledged as unconfirmable): AI PRs averaged 10.83 issues vs 6.45 for human PRs (~1.7x). Logic/correctness issues 75% more common; error-handling gaps nearly 2x; readability >3x; security up to 2.74x; excessive I/O ~8x; concurrency/dependency ~2x; formatting 2.66x; naming inconsistencies ~2x. "No issue category was unique to AI." — [CodeRabbit, vendor](https://www.coderabbit.ai/blog/state-of-ai-vs-human-code-generation-report)

**Duplicated code / reinvented helpers / no refactoring**
- HN 45278819, user gerash: agents are "reimplementing the same building blocks rather than refactoring and reusing because it's cheap to reimplement." — [HN, primary](https://news.ycombinator.com/item?id=45278819)
- HN 50059523, user jacquesm: "They tend to generate code that is repetitive. It misses opportunities for extraction and refactoring." User CharlieDigital: "it will happily duplicate some helper method from another test file rather than pulling it out and reusing it." — [HN, primary](https://news.ycombinator.com/item?id=50059523)
- Pragmatic Engineer (Gergely Orosz and Elin Nilsson, May 19 2026, subscriber survey): a CTO at a European startup said agents "generate too much and repetitive code, making systems harder to maintain"; the article lists "duplicated and verbose code, poor abstractions" among causes of declining codebase quality. — [Pragmatic Engineer via mirror, primary text](https://thepixelpioneers.blogspot.com/2026/05/ais-impact-on-software-engineers-in.html); original: https://newsletter.pragmaticengineer.com/p/ai-impact-on-software-engineers-part-2
- GitClear "AI Copilot Code Quality 2025" (211M changed lines, 2020–2024; foundational, data ends Dec 2024): moved (refactored) code fell from ~25% of changed lines in 2021 to <10% in 2024; copy/pasted code rose from 8.3% to 12.3%; headline "4x more code cloning"; "increases in short-term churn code." Correlational, not causal. — [GitClear, primary](https://www.gitclear.com/ai_assistant_code_quality_2025_research); [devclass summary, secondary](https://devclass.com/2025/02/20/ai-is-eroding-code-quality-states-new-in-depth-report/)

**Over-engineering / bloat / cruft**
- HN 45278819, user madamelic: "It drives me up the wall how these models try to one-shot production code and put a lot of cruft in." User gonational: "LLM coding does not discourage large piles of code, since the output is instantaneous." User greenavocado: "insane things like grossly violating each SOLID principle." — [HN, primary](https://news.ycombinator.com/item?id=45278819)
- HN 50059523, user yonaguska: "not every single edge case is relevant enough to warrant the additional loc or complexity introduced." User pllbnk: models "try to mimic the code they have been trained on and incorporate these edge cases into each fragment." User yaanfpv: "Can we also talk about the infinite tests and code comments that agents write?" — [HN, primary](https://news.ycombinator.com/item?id=50059523)

**No unifying design / lost intent**
- HN 45278819, user roughly: "LLM-generating code has no unifying theory behind it - every line may as well have been written by a different person" and "I've wasted hours trying to understand someone's MR, only to realize it's vibe code and there's no reason for any of it." User barrell: "The maintenance burden of code that has no real author is felt months/years after the code in written." — [HN, primary](https://news.ycombinator.com/item?id=45278819)
- Margaret-Anne Storey (professor, University of Victoria), "cognitive debt": "Even if AI agents produce code that could be easy to understand, the humans involved may have simply lost the plot." — [LeadDev, Mar 3 2026, secondary quoting Storey](https://leaddev.com/ai/you-cant-verify-all-the-ai-generated-code)

**Inconsistent style**
- HN 45278819, user dgunay: "it usually looks and feels just slightly stylistically foreign compared to the code around it." / "I personally edit mine before code review so that it looks how I would have written it." — [HN, primary](https://news.ycombinator.com/item?id=45278819)
- CodeRabbit: formatting issues 2.66x, naming inconsistencies ~2x in AI PRs. — [CodeRabbit, vendor](https://www.coderabbit.ai/blog/state-of-ai-vs-human-code-generation-report)

**Security**
- Veracode 2025 GenAI Code Security Report (July 30 2025; 80 coding tasks, 100+ LLMs): ~45% of generated samples failed security tests / contained OWASP Top 10 weaknesses; Java failure rate >70%; Python/C#/JavaScript 38–45%; XSS undefended in 86% of relevant samples; security performance "remaining unchanged over time" despite newer models. — [Veracode press release, primary](https://www.veracode.com/press-release/ai-generated-code-poses-major-security-risks-in-nearly-half-of-all-development-tasks-veracode-research-reveals/); [Help Net Security, secondary](https://www.helpnetsecurity.com/2025/08/07/create-ai-code-security-risks/)
- Christian Kästner (Carnegie Mellon): in a class assignment on fixing an AI agent's security flaw, ~80% of students lost points because they trusted the agent's broken fix. — [LeadDev, secondary](https://leaddev.com/ai/you-cant-verify-all-the-ai-generated-code)
- HN 50059523, user zmmmmm on what review must now catch: "Employees inserting back doors, bypassing tests, 'cheating' on design decisions to get the work done faster." — [HN, primary](https://news.ycombinator.com/item?id=50059523)

**Author can't explain the PR / skipped self-verification**
- HN 45278819, user skydhash: "we trust our colleagues to test and verify their own work. Which AI users usually skip." User nharada: "My biggest annoyance is that people aren't transparent about when they use AI." — [HN, primary](https://news.ycombinator.com/item?id=45278819)
- HN "A standard protocol to handle and discard low-effort, AI-Generated pull requests" (305 points, 113 comments, ~Mar 2026, linked to https://406.fail/). User lelanthran: "If your PR took less time to create and submit than it takes the maintainer to read, then you didn't read your own PR!" User zephyruslives: "if the maintainers cannot hold the human accountable for the AI generated code, then it shouldn't be accepted." Ghostty's AI_POLICY.md quoted: "If you can't explain what your changes do and how they interact with the greater system without the aid of AI tools..." — [HN, primary](https://news.ycombinator.com/item?id=47267947)
- Pragmatic Engineer May 2026: principal DevOps engineer at a large European company: "In our company, we hand AI tools to inexperienced engineers who can't distinguish good code from bad code." — [Pragmatic Engineer via mirror](https://thepixelpioneers.blogspot.com/2026/05/ais-impact-on-software-engineers-in.html)

**Counterpoint**
- HN 45278819, user duxup: "I'm really not seeing a lot of code that I can say is bad AI code." — [HN, primary](https://news.ycombinator.com/item?id=45278819)
- METR (July 10 2025 RCT): developers "submitted PRs of similar quality with and without AI" per the blog summary. — [METR, primary](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/)

### Inferences
- The failure modes cluster into two groups a tool could target: (a) *local* smells reviewers can see but are too tired to flag (duplication, bloat, style drift, naming) and (b) *non-local* defects reviewers structurally cannot see in a diff (cross-file logic, missing edge cases, silent partial handling, convention violations). Complaints about (b) are the most emotionally charged.
- Vendor datasets (CodeRabbit, Greptile, Veracode, Sonar) and community threads agree on direction even though they have obvious incentives; the HN quotes are the most credible "voice of the reviewer" material.

### Gaps
- Could not retrieve the text of the r/ExperiencedDevs thread "AI slop PRs are burning me and my team out hard" (reddit.com and old.reddit.com blocked). It is the primary citation in the arXiv "Endless Stream of AI Slop" paper: https://www.reddit.com/r/ExperiencedDevs/comments/1kr8clp/ai_slop_prs_are_burning_me_and_my_team_out_hard/
- No per-tool breakdown (Claude Code vs Cursor vs Copilot vs Codex vs Devin vs Windsurf) of failure modes was found in any source; complaints are almost always about "AI"/"agents" generically.
- Godot maintainers' "draining and demoralizing" statement (Feb 2026) was only seen via secondary coverage: https://borecraft.com/2026/02/18/a-recent-rise-in-ai-slop-code-submissions-is-draining-and-demoralizing-the-team-behind-open-source-game-engine-godot/ — primary Godot post not located.

---

## Key question 2: How have PR volume and review load changed (with numbers)?

### Takeaway
Every quantitative source agrees: output per developer is up sharply (roughly +20% to +98% PRs depending on dataset) while review time has grown faster than output (+91% in Faros 2025; median time-in-review +441% in Faros 2026), PR size has grown, and defect/incident rates per PR are rising. Review, not generation, is now the bottleneck, and some teams respond by merging without review.

### Cited Findings

**Faros AI "AI Productivity Paradox" (vendor telemetry; July 23 2025; 10,000+ developers, 1,255 teams)**
- High-AI-adoption teams complete "21% more tasks" and merge "98% more pull requests"; "PR review time increases 91%"; "154% increase in average PR size"; "9% increase in bugs per developer"; developers touch "47% more pull requests" per day; "no significant correlation between AI adoption and company-level improvements." — [Faros 2025, vendor/primary](https://www.faros.ai/blog/ai-productivity-paradox-report-2025)
- Note: some Faros FAQ pages cite 51.3% for PR-size increase instead of 154%; the main report page says 154%. Flagging as unresolved inconsistency within Faros's own site. — [Faros FAQ page citing 51.3%, via search summary](https://www.faros.ai/blog/key-takeaways-from-the-dora-report-2025)

**Faros AI "AI Engineering Report 2026: Acceleration Whiplash" (vendor telemetry; Apr 12 2026; 22,000 developers, 4,000+ teams)**
- Median time to first PR review "up 156.6%"; average time in code review "up 199.6%"; median time in review "up 441.5%"; PRs merged without any review "up 31.3%"; incidents-to-PR ratio "up 242.7%"; monthly incidents "up 57.9%"; bugs per developer "has risen to 54%" (vs 9% in 2025 report); lines-deleted-to-added ratio "increased 861%"; PR merge rate per developer "up 16.2%"; task throughput "up 33.7%"; acceptance rate of AI code rose from 20% to 60%; daily PR contexts per developer "up 67.4%"; work restarts "up 13.8%." — [Faros 2026, vendor/primary](https://faros.ai/blog/ai-acceleration-whiplash-takeaways)
- Caveat: 2025 and 2026 are different datasets; do not chain the numbers.

**DORA 2025 "State of AI-assisted Software Development" (Google Cloud; primary PDF not fetched, figures via DORA's own follow-up article)**
- "higher AI adoption is associated with an increase in both software delivery throughput and software delivery instability" (reversal of 2024 on throughput; instability persists). — [DORA "Balancing AI tensions", Mar 10 2026, primary](https://dora.dev/insights/balancing-ai-tensions/)
- "The time saved in creation is frequently re-allocated to auditing and verification." / "90% of technology professionals now use AI at work, and over 80% believe it has increased their productivity." / "30% of developers currently report little to no trust in the code generated by AI." — [DORA, primary](https://dora.dev/insights/balancing-ai-tensions/)
- Engineer quotes from the 1,110 Google-engineer open-ended responses (Q3 2025): "I feel somewhat more productive, but it's at a cost." / "Reviewing [another's] code is so much harder than writing it." / "AI tools increase my productivity, they write code faster than I could" but the code was lower quality than they could write. — [DORA, primary](https://dora.dev/insights/balancing-ai-tensions/)
- 2025 DORA report landing page: "AI's primary role is as an amplifier, magnifying an organization's existing strengths and weaknesses." — [DORA 2025 report page, primary](https://dora.dev/research/2025/dora-report/)

**GitHub Octoverse 2025 (Oct 28 2025)**
- "518.7M pull requests merged (+29% YoY)"; "43.2 million pull requests merged on average each month (+23% YoY)" (2024 monthly avg: 35M); Copilot coding agent: "1+ million pull requests that were created between May 2025 and September 2025"; "Governance is not keeping pace with velocity." The Octoverse page itself does not discuss AI slop or maintainer burden. — [GitHub Octoverse 2025, primary](https://github.blog/news-insights/octoverse/octoverse-a-new-developer-joins-github-every-second-as-ai-leads-typescript-to-1/)
- Independent tracker (Simon Willison citing PR Arena, Oct 1 2025): Claude Code ~146k total PRs / 123k merged; GitHub Copilot ~247k total / 152k merged; Codex ahead of Copilot in PR counts (autonomous-agent-tagged PRs only). — [Simon Willison, secondary](https://feeds.simonwillison.net/2025/Oct/1/prarena/)

**METR RCT (primary)**
- July 10 2025: 16 experienced open-source developers, 246 issues; AI-allowed tasks took "19% longer" (CI +2% to +39%); developers forecast 24% speedup and afterwards still believed they had been ~20% faster. Authors: Becker, Rush, Barnes, Rein. — [METR, primary](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/)
- Feb 24 2026 update: follow-up with 57 developers, 143 repos, 800+ tasks; returning developers -18% time (CI -38% to +9%), new developers -4% (CI -15% to +9%); METR says "the data from our new experiment gives us an unreliable signal" because "developers choosing not to participate in the study because they do not wish to work without AI" has increased; 30–50% of surveyed developers withheld some tasks; time tracking unreliable when "running multiple agents concurrently." — [METR update, primary](https://metr.org/blog/2026-02-24-uplift-update/)
- Secondary coverage of the 2025 study reports developers accepted <44% of generated code and spent ~9% of time cleaning AI output — not on METR's blog page; treat as secondary. — [DX newsletter, secondary](https://newsletter.getdx.com/p/metr-study-on-how-ai-affects-developer-productivity)

**Stack Overflow Developer Survey 2025 (secondary summaries)**
- 84% using or planning to use AI tools (76% in 2024); 46% actively distrust AI accuracy vs 33% trust (distrust was 31% in 2024); only ~3% "highly trust." Experienced devs: ~2.5–2.6% highly trust, ~20% highly distrust (8.3% in 2024). — [TechRepublic, secondary](https://www.techrepublic.com/article/news-developer-trust-in-ai-declines/); [RDEL, secondary](https://rdel.substack.com/p/rdel-105-top-findings-from-the-2025)

**Sonar 2026 State of Code (1,100+ devs, Jan 8 2026)**
- AI accounts for 42% of committed code (expected 65% by 2027); 96% do not fully trust AI code is functionally correct; only 48% always check AI-assisted code before committing; 38% say reviewing AI code takes more effort than reviewing a human colleague's; toil flat at 24% of the week regardless of AI use; 35% use AI tools via personal accounts. — [Sonar, vendor/primary survey](https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/)

**Anecdotal load numbers from practitioners**
- HN 50059523: user atleastoptimal: "instead of reviewing 1,000 lines a day I now 'review' 10,000 lines a day." User PacificSpecific: a friend left a job because "80% of the job was reviewing Claude submissions." User onder_ceylan: "With 10s of PRs waiting in the pipeline to be reviewed, judged and tested." User omellet: "Code can be created faster but that has moved the bottleneck to review." User hyperpape quoting the linked paper: baseline 5,017 LOC/month, estimated increase 1,494 lines per worker-month; baseline 5.34 PRs/month, +1.22; baseline code-review time 7.03 days. — [HN, primary](https://news.ycombinator.com/item?id=50059523)
- HN 45278819: user breppp: "there's an asymmetry between the time it takes to generate crap code and the time it takes to review crap code." User nharada: "It's a huge amount of work offloaded on me, the reviewer." — [HN, primary](https://news.ycombinator.com/item?id=45278819)
- Bryan Finster: AI raised his code production "a factor of three or more" and the review process "did not hold up"; citing SmartBear/Cisco data, past 400 lines of diff "you are not getting a review. You are getting a rubber stamp." — [Finster, primary](https://bryanfinster.substack.com/p/ai-broke-your-code-review-heres-how)
- Tom Finch (engineering leader, Chainguard): "humans are now being asked to manually verify more code than is realistically possible." Chainguard research: engineers spend only 16% of their week building features; 86% of UK developers spend more time maintaining/patching than creating. — [LeadDev, secondary](https://leaddev.com/ai/you-cant-verify-all-the-ai-generated-code)
- Pragmatic Engineer May 2026: lead engineer at a small company: "We're at the death of code review." / "I have no motivation in spending that time to review a giant PR where it's clear that even the original author [did not]." Engineering lead at a 10,000+ person European company: "We've also seen costly production incidents caused by code written and/or approved by AI." Senior manager: "with AI-driven development we introduce more bugs than before." — [Pragmatic Engineer via mirror](https://thepixelpioneers.blogspot.com/2026/05/ais-impact-on-software-engineers-in.html)
- Open-source: HN 47267947 user jerf: "Code used to be rare, and therefore, worth a lot. Now it's not rare." / "I'd honestly rather have an Issue filed with your proposed prompt in it than the actual output of your AI." User lelanthran describing spam: "you submit 200 PRs per day to 200 different projects." — [HN, primary](https://news.ycombinator.com/item?id=47267947)

### Inferences
- Review time grows super-linearly with PR volume because PRs also got bigger (Faros: +154% size) and reviewers' per-line attention drops past ~400 lines (Finster/SmartBear). This is why "+98% PRs" produces "+91% review time" in 2025 and "+441% median time in review" in 2026.
- The +31.3% rise in unreviewed merges (Faros 2026) plus "We haven't done manual code review for months" (HN user eivindmeyer) suggests some teams are resolving the bottleneck by abandoning review rather than scaling it, which pairs with rising incident rates.
- METR's headline "19% slower" is widely quoted but METR itself now considers its measurements unreliable; use it as evidence of the perception gap (devs believe +20% while measured -19%) rather than as a stable productivity number.

### Gaps
- DORA 2025 full report PDF not fetched; sample size and exact instability figures not verified against the primary document (only via DORA's own Mar 2026 article).
- The "Cortex report" cited by CodeRabbit (PRs per author +20% YoY, incidents per PR +23.5%) could not be located as a primary source.
- No Google "Impact of AI on software engineering" 2025 paper with that title was found; closest Google sources are the ICSE 2025 RCT (96 engineers, faster on task, no review-time measurement) and the 2024 "AI in software engineering at Google" blog (foundational, not 2025). https://research.google/blog/ai-in-software-engineering-at-google-progress-and-the-path-ahead/
- LinearB and JetBrains State of Developer Ecosystem data on review load were not reached within the call budget.
- The paper behind the HN "more code, but not more software" thread (Ars Technica, ~Oct 2026) was not fetched directly; its numbers are quoted via commenters.

---

## Key question 3: What do reviewers say they can no longer catch, and what do they wish tooling would catch?

### Takeaway
Reviewers say they can no longer do line-by-line review at agent volume and therefore miss cross-file logic errors, silent edge-case drops, convention violations, duplication, and security regressions. What they ask tooling for is mechanical gating (lintable restrictions, size limits, required failing-test-then-passing evidence, invariants files, acceptance criteria) so humans can keep judgment for intent, design, and security.

### Cited Findings

**What they can no longer catch**
- Finster: past 400 lines "you are not getting a review. You are getting a rubber stamp"; review is "a low-frequency check on high-frequency production"; "If things are changing rapidly, you need to check them even more rapidly." — [Finster, primary](https://bryanfinster.substack.com/p/ai-broke-your-code-review-heres-how)
- HN 50059523, user onder_ceylan on cognitive load: "A human's working memory ... holds about 4 to 7 items (or 'chunks' of information) at one time." User jack_sunsetless: "Fewer people on the team understand what's actually in there, so there are more and more bugs." User staminade: "Code review was always a significant bottleneck and is even more so with agentic coding" and "senior engineers find it very difficult to give up on line-by-line review." User brabel (old joke, now literal): "a PR with 10 lines changes got 10 comments (all nitpicking), a PR with 1000 lines got just one, LGTM." — [HN, primary](https://news.ycombinator.com/item?id=50059523)
- Greptile: cross-file logic issues are "hard to spot without understanding the rest of the codebase." — [Greptile, vendor](https://greptile.com/blog/ai-code-review)
- HN 50059523, user bunderbunder on AI reviewers: "I haven't really seen much evidence of AI code review catching either of these problems" (intent/design-level defects). — [HN, primary](https://news.ycombinator.com/item?id=50059523)
- Aviator blog argues AI reviewing AI produces "circular validation" because generator and reviewer share training patterns. — [Aviator, vendor](https://www.aviator.co/blog/ai-code-review-is-still-a-review/)
- Werner Vogels (AWS CTO, re:Invent 2025, via LeadDev/Sonar): when humans write code comprehension comes during creation; when machines write it, comprehension "has to be rebuilt during review" — "verification debt." (No verbatim keynote quote located.) — [LeadDev, secondary](https://leaddev.com/ai/you-cant-verify-all-the-ai-generated-code); [Sonar press release](https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/)

**What they wish tooling / process would catch**
- HN 50059523, user dathinab: "rely on lintable language specific limitations (e.g. rust no unsafe, typing, white/banned lists of imports, no eval" and "give it a short not in depth human review, except for anything security critical." User virpo: an INVARIANTS.md file — "The agent checks every merge against it, and changing a line in it is only my call." User jpurnell: "Where I'm probably most strict in the tooling is on documentation, which makes the code review much more manageable." User fasterik: "have human accountability on the API design and tests, not on line-by-line source code." User zmmmmm: "I don't need to syntactically parse every line to scan it for the type of risks that matter here." User yunwal: "eventually we'll substitute code review with statistical evals for some cases." — [HN, primary](https://news.ycombinator.com/item?id=50059523)
- HN 47267947 (maintainers): user ramon156: "If its a bug, the PR should have a red line to confirm its fixed" / "If its a feature, i want acceptance criteria at least." User JoshTriplett: "If you assert this is not AI, insert $10." User quotemstr: "some 'skin in the game' approach can address AI slop spam." User deckar01: "Please rate limit unregistered bots." Policies adopted: Ghostty AI_POLICY.md; disabling PRs for non-collaborators; blocking low-effort submitters. — [HN, primary](https://news.ycombinator.com/item?id=47267947)
- arXiv "An Endless Stream of AI Slop" (Baltes, Cheong, Treude; v1 Mar 28 2026; 1,154 Reddit+HN posts; codebook of 15 codes in clusters Review Friction / Quality Degradation / Forces and Consequences; frames slop as a "tragedy of the commons"): proposed mitigations include enforcing maximal PR sizes, mandatory self-review, synchronous walkthroughs, and dual-team reviews. — [arXiv, primary](https://arxiv.org/abs/2603.27249)
- Finster: review should become "a quick validation that the solution aligns with the specification that the team agreed on," with quality from automated checks and team-defined acceptance tests written before coding; humans "reviewing agent findings instead of raw diffs." — [Finster, primary](https://bryanfinster.substack.com/p/ai-broke-your-code-review-heres-how)
- Vendor claims on what AI review catches (self-reported, unverified): Greptile Q3 2026: 4.8M PRs reviewed, 65% had at least one flagged issue, 61% of comments led to a code change; ~17M issues since Mar 2026: 1% P0, 52% defects, 47% quality. — [Greptile, vendor](https://greptile.com/blog/ai-code-review)

### Inferences
- Reviewers draw a line between what should be *gated* (size, lint, banned constructs, duplication, conventions, test evidence) and what should remain *human* (intent, API design, security-critical paths). Tooling that takes the first category off their plate is what they are asking for; tooling that adds another stream of comments to skim is not.
- Several practitioners explicitly want "review the agent's findings, not the raw diff" — a shift from diff-centric to evidence-centric review.

### Gaps
- No survey question found that directly asks "what do you wish review tooling caught"; the above is reconstructed from forum threads and essays.

---

## Key question 4: Which complaints are about the model, and which about the process?

### Takeaway
Model-level complaints: subtle logic errors, duplication, bloat, foreign style, no design intent, insecure defaults. Process-level complaints (arguably the louder set in 2026): authors not self-reviewing or testing, no transparency about AI use, giant PRs, no gates, reviewers rubber-stamping or skipping review entirely, and juniors unable to judge output. Most practitioners and DORA frame the outcome as process amplification ("AI is an amplifier").

### Cited Findings

**Model**
- Subtle/edge-case bugs, cross-file logic, 95%-handling (Greptile; HN juancn, dathinab); repetitive/un-refactored code (HN jacquesm, gerash, CharlieDigital; GitClear); cruft and SOLID violations (HN madamelic, greenavocado); stylistically foreign (HN dgunay); no unifying theory (HN roughly); insecure 45% of the time with no improvement across model generations (Veracode). Sources as cited in Q1.
- HN 45278819, user bluGill: "If AI is writing such code I don't trust them to figure out those details." — [HN, primary](https://news.ycombinator.com/item?id=45278819)

**Process**
- Authors skipping verification: HN skydhash "we trust our colleagues to test and verify their own work. Which AI users usually skip."; Sonar: only 48% always check AI code before committing; HN lelanthran "you didn't read your own PR!" — [HN 45278819](https://news.ycombinator.com/item?id=45278819); [Sonar](https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/); [HN 47267947](https://news.ycombinator.com/item?id=47267947)
- No transparency: HN nharada "people aren't transparent about when they use AI." — [HN 45278819](https://news.ycombinator.com/item?id=45278819)
- Rubber-stamping / skipping review: Faros 2026 unreviewed merges +31.3%; HN eivindmeyer "We havent done manual code review for months actually."; HN fortzi "What you describe sounds like little more than rubber stamping a PR"; HN jacquesm "as long as 'the tests pass' it is all good."; HN leecommamichael "the team committing it won't perceive a negative outcome, whether it has been had or not." — [Faros 2026](https://faros.ai/blog/ai-acceleration-whiplash-takeaways); [HN 50059523](https://news.ycombinator.com/item?id=50059523)
- Giant PRs / batch size: Faros 2025 PR size +154%; Finster 400-line threshold; Pragmatic Engineer "giant PR" quote; Blind thread describing "multi-thousand line diffs." — [Faros 2025](https://www.faros.ai/blog/ai-productivity-paradox-report-2025); [Finster](https://bryanfinster.substack.com/p/ai-broke-your-code-review-heres-how); [Blind, secondary](https://www.teamblind.com/post/how-do-you-effectively-call-out-ai-slop-e4isq8og)
- Who gets the tools: Pragmatic Engineer principal DevOps engineer: "we hand AI tools to inexperienced engineers who can't distinguish good code from bad code." — [Pragmatic Engineer via mirror](https://thepixelpioneers.blogspot.com/2026/05/ais-impact-on-software-engineers-in.html)
- Reviewer social cost: a DEV post notes reviewers who leave many comments get labeled "too picky" or "blocking progress" (secondary, opinion). — [DEV, secondary](https://dev.to/sizan_mahmud0_e7c3fd0cb68/the-hidden-cost-of-ai-coding-tools-why-generate-and-submit-is-destroying-code-quality-46o)
- Organizational framing: DORA 2025: "AI's primary role is as an amplifier, magnifying an organization's existing strengths and weaknesses." — [DORA](https://dora.dev/research/2025/dora-report/)
- Open-source accountability: HN zephyruslives "if the maintainers cannot hold the human accountable for the AI generated code, then it shouldn't be accepted." — [HN 47267947](https://news.ycombinator.com/item?id=47267947)
- Value of review being lost (process): HN cobbal "PRs are an opportunity to learn (both committer and reviewer), communicate, and improve the quality of code"; HN strogonoff on losing the "good-faith, succinct and quick 'why' sort of discussion." — [HN 50059523](https://news.ycombinator.com/item?id=50059523); [HN 47267947](https://news.ycombinator.com/item?id=47267947)

### Inferences
- Practitioners increasingly blame process more than model: the same model output is acceptable when the author self-reviews, splits PRs, and runs gates, and toxic when "generate and submit." That makes "we gate what the author skipped" a credible product promise, whereas "we make the model write better code" is not what reviewers are asking for.
- The "amplifier" framing from DORA gives a non-accusatory way to talk about process debt.

### Gaps
- No controlled study isolating process interventions (PR size limits, mandatory self-review) on AI-code defect rates was found.

---

## Key question 5: What phrases do they use that a landing page could echo or should avoid?

### Takeaway
The vernacular is vivid and mostly negative. "AI slop," "vibe coding," "verification debt," "cognitive debt," "rubber stamp," "review bottleneck," "babysitting," and "looks right but isn't / almost right" are the phrases with the widest currency. "AI slop" and "vibe coding" are loaded and partisan; "verification debt," "review bottleneck," and "almost right" are the safer, more widely accepted framings.

### Cited Findings

Phrases in active use (with origin/attribution):
- "AI slop" — pervasive; subject of an academic paper (Baltes/Cheong/Treude, 2026) that defines it as "low-quality AI-generated content" and names the cluster "Review Friction." — [arXiv](https://arxiv.org/abs/2603.27249). Thread titles: "AI slop PRs are burning me and my team out hard" (r/ExperiencedDevs); "AI splattered code" (HN). — [HN](https://news.ycombinator.com/item?id=45278819)
- "vibe coding" / "vibe code" — used pejoratively by reviewers: "only to realize it's vibe code and there's no reason for any of it." — [HN](https://news.ycombinator.com/item?id=45278819)
- "verification debt" — Werner Vogels (AWS CTO), picked up by Sonar, Chainguard (Tom Finch), LeadDev. — [LeadDev](https://leaddev.com/ai/you-cant-verify-all-the-ai-generated-code); [Sonar](https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/)
- "verification gap" — Sonar press-release headline. — [Sonar](https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/)
- "cognitive debt" — Margaret-Anne Storey: "technical debt lives in the code, cognitive debt lives in the minds of the developers." — [LeadDev](https://leaddev.com/ai/you-cant-verify-all-the-ai-generated-code)
- "almost right, but not quite" — Stack Overflow 2025 survey wording; 66% top frustration. — [RDEL](https://rdel.substack.com/p/rdel-105-top-findings-from-the-2025)
- "looks correct but isn't reliable" — Sonar (53%). "looks right" — Greptile, Finster. — [Sonar](https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/); [Greptile](https://greptile.com/blog/ai-code-review)
- "believable crap" — HN juancn. — [HN](https://news.ycombinator.com/item?id=45278819)
- "rubber stamp" — Finster; HN fortzi. — [Finster](https://bryanfinster.substack.com/p/ai-broke-your-code-review-heres-how); [HN](https://news.ycombinator.com/item?id=50059523)
- "review bottleneck" / "the bottleneck moved to review" — HN omellet, staminade; Faros. — [HN](https://news.ycombinator.com/item?id=50059523); [Faros](https://www.faros.ai/blog/ai-productivity-paradox-report-2025)
- "babysitting the AI" — Google engineer in DORA qualitative data. — [DORA](https://dora.dev/insights/balancing-ai-tensions/)
- "death of code review" — lead engineer quoted by Pragmatic Engineer. — [Pragmatic Engineer via mirror](https://thepixelpioneers.blogspot.com/2026/05/ais-impact-on-software-engineers-in.html)
- "AI productivity paradox," "acceleration whiplash" — Faros report titles. — [Faros 2025](https://www.faros.ai/blog/ai-productivity-paradox-report-2025); [Faros 2026](https://faros.ai/blog/ai-acceleration-whiplash-takeaways)
- "AI is an amplifier" — DORA 2025. — [DORA](https://dora.dev/research/2025/dora-report/)
- "farm green squares" — maintainers describing spam PRs. — [HN](https://news.ycombinator.com/item?id=47267947)
- "code that has no real author" — HN barrell. — [HN](https://news.ycombinator.com/item?id=45278819)
- "drafting tool, not a shipping tool" — team-norm phrasing seen in a forum post (secondary). — [tianpan forum, secondary](https://tianpan.co/forum/t/our-team-generates-98-more-prs-with-ai-but-review-time-increased-91-were-drowning/3489)
- "review debt" — I found no source using this exact phrase; "verification debt" is the established term.

### Inferences
- Echo: "verification debt," "the bottleneck moved to review," "looks right but isn't," "almost right," "rubber stamp," "amplifier." These are used by neutral/authoritative voices (DORA, Vogels, Storey, Stack Overflow) and by practitioners alike.
- Avoid or use carefully: "AI slop" and "vibe coding" are tribal insults; using them in product copy signals a side and may alienate the agent-enthusiast half of a team. "Death of code review" is hyperbolic. "Babysitting" is relatable but demeaning to the author persona.
- Reviewer-side emotional words that recur: "burned out," "draining," "demoralizing," "drowning," "offloaded on me," "no motivation" — useful for empathy copy, but the data shows reviewers want fewer things to look at, not more comments.

### Gaps
- No systematic frequency count of these phrases exists beyond the arXiv paper's qualitative codebook (full text not fetched).

---

## Source index (primary first)

- HN: "Ask HN: Is anyone else sick of AI splattered code" (Sept 2025) — https://news.ycombinator.com/item?id=45278819
- HN: "AI coding agents generate more code, but not more software" (~Oct 2026) — https://news.ycombinator.com/item?id=50059523
- HN: "A standard protocol to handle and discard low-effort, AI-Generated pull requests" (~Mar 2026) — https://news.ycombinator.com/item?id=47267947
- arXiv: Baltes, Cheong, Treude, "An Endless Stream of AI Slop" (Mar 2026) — https://arxiv.org/abs/2603.27249
- DORA: "Balancing AI tensions" (Mar 10 2026) — https://dora.dev/insights/balancing-ai-tensions/ ; 2025 report page — https://dora.dev/research/2025/dora-report/
- METR: July 10 2025 study — https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/ ; Feb 24 2026 update — https://metr.org/blog/2026-02-24-uplift-update/
- GitHub Octoverse 2025 — https://github.blog/news-insights/octoverse/octoverse-a-new-developer-joins-github-every-second-as-ai-leads-typescript-to-1/
- Faros AI 2025 — https://www.faros.ai/blog/ai-productivity-paradox-report-2025 ; 2026 — https://faros.ai/blog/ai-acceleration-whiplash-takeaways
- Sonar 2026 State of Code press release (Jan 8 2026) — https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/
- Veracode 2025 GenAI Code Security Report (Jul 30 2025) — https://www.veracode.com/press-release/ai-generated-code-poses-major-security-risks-in-nearly-half-of-all-development-tasks-veracode-research-reveals/
- CodeRabbit (Dec 17 2025) — https://www.coderabbit.ai/blog/state-of-ai-vs-human-code-generation-report
- GitClear 2025 — https://www.gitclear.com/ai_assistant_code_quality_2025_research
- Greptile guide — https://greptile.com/blog/ai-code-review
- Bryan Finster (Mar 4 2026) — https://bryanfinster.substack.com/p/ai-broke-your-code-review-heres-how
- LeadDev, "You can't verify all the AI-generated code" (Mar 3 2026) — https://leaddev.com/ai/you-cant-verify-all-the-ai-generated-code
- Pragmatic Engineer, "AI's impact on software engineers in 2026, Part 2" (May 19 2026; read via mirror) — https://newsletter.pragmaticengineer.com/p/ai-impact-on-software-engineers-part-2
- Stack Overflow 2025 survey (via TechRepublic / RDEL summaries) — https://www.techrepublic.com/article/news-developer-trust-in-ai-declines/ ; https://rdel.substack.com/p/rdel-105-top-findings-from-the-2025
- Simon Willison on PR Arena (Oct 1 2025) — https://feeds.simonwillison.net/2025/Oct/1/prarena/
