# The `npx skills` manager: what this repo relies on

Findings for `skills` 1.7.6 (vercel-labs/skills), checked on 2026-11-03
against its docs and its published `dist/cli.mjs`, then by installing this
repo from a local path into a scratch HOME and project.

## Discovery in a repository

- If the repository root has a `SKILL.md`, that is the only skill found and
  the search stops (unless `--full-depth`). So this repo must **not** have a
  root `SKILL.md`, or the whole repo would become the skill.
- Otherwise it searches container directories: `skills/`, `skills/.curated/`,
  `skills/.experimental/`, `skills/.system/` and every agent's project dir
  (`.claude/skills/`, `.agents/skills/`, ...). Each container is walked up to
  three levels (`skills/<name>/SKILL.md`, `skills/<cat>/<name>/SKILL.md`, ...);
  a shallower `SKILL.md` shadows deeper ones.
- Extra containers can be listed in `skills.sh.json` (`"paths": ["./x"]`) at
  the root; `.claude-plugin/marketplace.json` / `plugin.json` skill paths are
  also read. Only when nothing is found does it fall back to a recursive search.
- Duplicate names: the first one found wins.

This repo uses `skills/coderefinery/SKILL.md` (no manifest needed).

## Frontmatter

- Required by the manager: `name` and `description`, both non-empty strings;
  otherwise the skill is skipped ("missing required frontmatter field(s)").
- Install directory name = `name` lowercased, runs of characters outside
  `[a-z0-9._]` turned into `-`.
- Optional: `metadata.internal: true` hides a skill unless
  `INSTALL_INTERNAL_SKILLS=1`.
- The Agent Skills spec (agentskills.io) adds: `name` is 1-64 chars of
  `a-z0-9-`, no leading/trailing/double hyphen, and matches the directory;
  `description` up to 1024 chars; optional `license`, `compatibility` (up to 500),
  `metadata` (string map), `allowed-tools`.
- Claude Code: every field is optional, unknown fields are ignored;
  `description` (+ `when_to_use`) is truncated at 1536 chars in the listing;
  `<` and `>` are escaped. It substitutes `${CLAUDE_SKILL_DIR}` (the directory
  holding `SKILL.md`) in skill content, which is how `SKILL.md` points at
  `scripts/coderefinery` regardless of the working directory.

## What gets installed

- The whole skill directory is copied recursively (`scripts/`, `LICENSE`,
  anything else), except `metadata.json` files and `.git`, `__pycache__`,
  `__pypackages__` directories. File modes are kept (`chmod` to the source
  mode), so `scripts/coderefinery` stays executable. Symlinks pointing outside the
  skill directory are skipped.
- GitHub sources are fetched with a shallow `git clone`, so the executable bit
  must be committed (git mode 100755) for the launcher to stay executable.
- Project scope (default) installs into `./<agent dir>/skills/<name>/`; `-g`
  installs into the agent's global dir. Claude Code: `.claude/skills/` and
  `~/.claude/skills/` (`$CLAUDE_CONFIG_DIR` honoured). Codex, Cursor (project),
  OpenCode, Gemini CLI, GitHub Copilot and the other "universal" agents:
  `.agents/skills/`. Full table: the manager's `docs/agents.md`.
- With one agent the files are copied into its directory. With several, a
  canonical copy goes to `.agents/skills/<name>/` and the other agents get a
  symlink to it (`--copy` forces copies).
- Project installs write `skills-lock.json` in the project (source, hash).
  Global installs track state in `$XDG_STATE_HOME/skills/.skill-lock.json`
  or `~/.agents/.skill-lock.json`.

## Commands used for this repo

```bash
npx skills add matheusAle/coderefinery                    # interactive: pick agents
npx skills add matheusAle/coderefinery -a claude-code -y  # project, Claude Code only
npx skills add matheusAle/coderefinery -g -a claude-code  # all projects
npx skills add ./path/to/coderefinery --list              # local checkout: list skills
npx skills add ./path/to/coderefinery -a claude-code -y   # local checkout: install
```

Other source forms: full GitHub URL, `.../tree/<ref>/skills/coderefinery`, any git
URL, GitLab/Azure URLs, a direct `SKILL.md` or archive URL. Requires Node.js
22.20+. Telemetry is off with `DISABLE_TELEMETRY=1` or `DO_NOT_TRACK=1`.

## Verified locally

`skills add <local checkout> --list` finds exactly `coderefinery`.
`-a claude-code -y` produced `.claude/skills/coderefinery/{SKILL.md,LICENSE,scripts/coderefinery}`
(launcher still `-rwxr-xr-x`) plus `skills-lock.json`, and nothing in HOME.
`-a claude-code -a codex` produced `.agents/skills/coderefinery/` with
`.claude/skills/coderefinery -> ../../.agents/skills/coderefinery`. The installed
launcher ran `coderefinery --version` through `CODEREFINERY_BIN`, `PATH`, a download from a
local release directory into the cache, and the cache.

## Sources

- https://github.com/vercel-labs/skills (README.md)
- https://github.com/vercel-labs/skills/blob/main/docs/creating-skills.md (frontmatter, discovery)
- https://github.com/vercel-labs/skills/blob/main/docs/commands.md (`add`, sources, scopes, symlink vs copy)
- https://github.com/vercel-labs/skills/blob/main/docs/agents.md (per-agent paths)
- https://github.com/vercel-labs/skills/blob/main/docs/troubleshooting.md
- npm `skills@1.7.6`, `dist/cli.mjs`: `discoverSkills`, `parseSkillMd`, `copyDirectory`, `sanitizeName`
- https://agentskills.io/specification
- https://code.claude.com/docs/en/skills (frontmatter, `${CLAUDE_SKILL_DIR}`)

## Supporting files (progressive disclosure)

Checked on 2026-11-04 against the Claude Code skills page, Anthropic's skill
authoring best practices and the Agent Skills spec. The skill uses this layout:
`SKILL.md` (rules, gate order, how to read each gate, exit codes),
`references/*.md` (per-language, testing, react, install details) and
`scripts/coderefinery` (the launcher, run and never read).

- Loading: only `name` and `description` are loaded at startup. The `SKILL.md`
  body loads when the skill is invoked. Other files load only when the agent
  reads them, so `SKILL.md` must say what each file holds and when to open it.
  Scripts are executed, not loaded.
- Size: keep the `SKILL.md` body under 500 lines (spec: under ~5000 tokens).
  A reference file over 100 lines gets a table of contents at the top, because
  an agent may preview it with a partial read (`head -100`).
- Links: relative paths from the skill root (`references/python.md`), forward
  slashes, one level deep. Every reference is linked from `SKILL.md`; a reference
  may name a sibling (`react.md`), which `SKILL.md` links too, never a deeper file. `references/` is the spec's
  conventional directory name; files are named by domain.
- Paths: Claude Code substitutes `${CLAUDE_SKILL_DIR}` (the directory holding
  `SKILL.md`) in skill content and in `allowed-tools` Bash rules, so commands
  work from any working directory. Other agents resolve paths relative to the
  skill directory.
- Installing: `npx skills` copies the whole skill directory recursively (see
  above), so `references/` needs no manifest. Verified 2026-11-04: installing
  this worktree with `skills@1.7.6 add <path> -a claude-code -y` into a scratch
  HOME and project produced `.claude/skills/coderefinery/` with `SKILL.md`,
  `LICENSE`, the nine `references/*.md` and `scripts/coderefinery` (still
  `-rwxr-xr-x`), `diff -r` identical to `skills/coderefinery/`.

Sources:

- https://code.claude.com/docs/en/skills ("Supporting files", `${CLAUDE_SKILL_DIR}`, frontmatter)
- https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices
  ("Progressive disclosure patterns", "Avoid deeply nested references",
  "Structure longer reference files with a table of contents", "Runtime environment")
- https://agentskills.io/specification ("Optional directories", "Progressive disclosure", "File references")
