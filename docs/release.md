# Releases: distribution, signing, workflows

How a coderefinery release is built, signed, stored and installed, and what
the owner sets up once.

## Where releases live

| what | where |
|---|---|
| storage | Cloudflare R2 bucket `coderefinery-releases` (account `d16dfc846cc4d6600745e57b7cebe94c`) |
| public URL | `https://coderefinery.dev/dl/<key>`, served by the Pages Function `site/functions/dl/[[path]].ts` (R2 binding `RELEASES`, declared in `site/wrangler.toml`) |
| installer | `https://coderefinery.dev/install.sh` = `site/public/install.sh` |
| mirror | the GitHub release of the tag (the skill launcher's fallback) |

Bucket layout:

```
latest.json                                   the newest release (written last)
vX.Y.Z/coderefinery_X.Y.Z_<os>_<arch>.tar.gz  make dist archives: darwin|linux × amd64|arm64
vX.Y.Z/SHA256SUMS                             shasum -a 256 of the archives
vX.Y.Z/SHA256SUMS.sig                         base64 ed25519 signature of SHA256SUMS
```

`latest.json`:

```json
{
  "version": "v1.2.3",
  "published": "2026-11-05T12:00:00Z",
  "assets": {
    "darwin-arm64": { "file": "coderefinery_1.2.3_darwin_arm64.tar.gz", "sha256": "…" },
    "linux-amd64":  { "file": "coderefinery_1.2.3_linux_amd64.tar.gz",  "sha256": "…" }
  }
}
```

`go run ./tools/release manifest vX.Y.Z dist` writes it from `dist/SHA256SUMS`.

The function serves only `latest.json` and `vX.Y.Z[-pre]/<file>` (anything
else is a 404 that never reaches the bucket), GET and HEAD, with ETag / 304.
`latest.json` is cached for 60 s, versioned files are `immutable`.
R2 is not served through a bucket domain because the wrangler token cannot
write DNS; the Pages project already owns the site, so a Function was enough.

## Who reads what

| client | reads | checks |
|---|---|---|
| `install.sh` | `latest.json` (or `CODEREFINERY_VERSION`), the archive, `SHA256SUMS` | sha256 of the archive |
| `coderefinery update` | `latest.json`, `SHA256SUMS`, `SHA256SUMS.sig`, the archive | signature of `SHA256SUMS` with the built-in key, sha256 of the archive, the new binary's `--version` |
| skill launcher | `vX.Y.Z/` archive + `SHA256SUMS` from `coderefinery.dev/dl`, else the GitHub release | sha256 of the archive |

All three take `CODEREFINERY_DOWNLOAD_URL` (default `https://coderefinery.dev/dl`);
`tests/install_test.sh` fails if their defaults drift apart.

## Signing key

ed25519 over `SHA256SUMS`. The public key is the `publicKey` constant in
`internal/selfupdate/release.go`. The private key (base64 of the 32-byte seed)
exists **only** as the GitHub Actions secret `CODEREFINERY_SIGNING_KEY`: it was
piped from `tools/release keygen` straight into `gh secret set` and never
written to disk. It cannot be read back; nobody has a copy.

The `signing-key` job of `ci.yml` signs a probe with the secret and verifies it
with the built-in public key on every push to main and every pull request from
this repo, so a secret that does not match the code fails CI, not a release.

Rotation (key lost, leaked, or on schedule):

```bash
go run ./tools/release keygen | gh secret set CODEREFINERY_SIGNING_KEY   # prints "public key: …" on stderr
```

Put the printed public key in `publicKey`, commit, and release. Binaries
released before the rotation only trust the old key: their `coderefinery update`
refuses releases signed with the new one, so their users reinstall once with
`curl -fsSL https://coderefinery.dev/install.sh | sh` (sha256-checked over
HTTPS). Say so in the release notes.

## Workflows

| workflow | when | what |
|---|---|---|
| `ci.yml` | push to main, pull requests, manual | `make ci`, launcher and installer tests (Linux, macOS); signing-key check; parity 36/36 (macOS) |
| `release.yml` | tag `v*` | see below |
| `site.yml` | push to main touching `site/`, called by `release.yml`, manual on main; pull requests only build | `npm ci && npm run build` in `site/`, then `wrangler pages deploy dist --project-name coderefinery --branch main` |

`release.yml`, job by job:

1. **release** (Linux): all three secrets present, else it stops naming the
   missing one; tag is `vX.Y.Z[-pre]` and equals `CODEREFINERY_VERSION` in the
   launcher; `make ci`; launcher and installer tests; `make dist`; smoke of the
   linux/amd64 binary; sign `SHA256SUMS` and verify it with the built-in key;
   write `latest.json`; upload the archives, `SHA256SUMS` and `SHA256SUMS.sig`
   to `vX.Y.Z/` in R2; publish the GitHub release (mirror; on a re-run the
   assets are replaced); **then** upload `latest.json`, so it never names a
   file that is not there. Pre-releases (`-rc.1`) and tags older than the
   current `latest.json` leave it alone.
2. **site**: `site.yml` deploys the tag's site (install.sh, the function).
3. **verify** (Linux and macOS): installs with the served `install.sh` and
   checks `--version`; `coderefinery update --check` says up to date; the
   served `SHA256SUMS.sig` verifies with the built-in key; the launcher installs
   from R2, and with R2 pointed at nothing, from the GitHub release.

Actions are pinned by commit (first-party only); wrangler is pinned by npm
version (`WRANGLER` in the workflows).

To release: set `CODEREFINERY_VERSION` in `skills/coderefinery/scripts/coderefinery`,
merge, push the tag `vX.Y.Z`.

## Secrets and settings

| name | kind | set? | used by |
|---|---|---|---|
| `CODEREFINERY_SIGNING_KEY` | secret | yes (2026-11-05) | `ci.yml` signing-key, `release.yml` |
| `CLOUDFLARE_ACCOUNT_ID` | secret | yes: `d16dfc846cc4d6600745e57b7cebe94c` | `release.yml`, `site.yml` |
| `CLOUDFLARE_API_TOKEN` | secret | **no, the owner creates it** (below) | `release.yml`, `site.yml` |
| `CODEREFINERY_SITE_URL` | variable, optional | no | `release.yml`: base of install.sh and `/dl` for the checks, default `https://coderefinery.dev` |

### Cloudflare API token

A scoped token cannot be created from wrangler's OAuth login. In the dashboard:

1. https://dash.cloudflare.com/profile/api-tokens → **Create Token** →
   **Custom token** → **Get started**.
2. Token name: `coderefinery GitHub Actions`.
3. Permissions (two rows, **Add more** for the second):
   - `Account` · `Workers R2 Storage` · `Edit`
   - `Account` · `Cloudflare Pages` · `Edit`
4. Account Resources: `Include` · `M.matheus.ale@gmail.com's Account`.
   (No zone permissions are needed.)
5. Optional: Client IP filtering empty (GitHub runners have no fixed IPs);
   TTL as you like (if it expires, releases and site deploys stop with an
   authentication error).
6. **Continue to summary** → **Create Token**, copy the value, then:

```bash
gh secret set CLOUDFLARE_API_TOKEN --repo matheusAle/coderefinery   # paste the token at the prompt
```

The R2 permission of a custom token covers every bucket of the account (the
account also has `pdfxray-releases`); the form has no per-bucket choice.
Until the secret exists, `site.yml` deploys and `release.yml` fail at their
first step with `secret CLOUDFLARE_API_TOKEN is missing`.

### DNS for coderefinery.dev

The Pages project has `coderefinery.dev` and `www.coderefinery.dev` attached,
both **pending: "CNAME record not set"**, so today only
`https://coderefinery-21a.pages.dev` answers. In the dashboard: zone
`coderefinery.dev` → **Workers & Pages** → `coderefinery` → **Custom domains**
→ each domain → **Activate domain** (or DNS → Records → add `CNAME` `@` and
`www` → `coderefinery-21a.pages.dev`, proxied). Until then, set the repository
variable `CODEREFINERY_SITE_URL=https://coderefinery-21a.pages.dev` for the
release checks; the installer, the launcher and `coderefinery update` default
to `coderefinery.dev` and need the domain.
