#!/bin/sh
# Tests for site/public/install.sh, the `curl -fsSL https://coderefinery.dev/install.sh | sh`
# installer. Every case pipes the installer into sh under `env -i`, with a
# scratch HOME, TMPDIR and install dir and a PATH of base tools only, against a
# file:// release laid out like the R2 bucket (latest.json, vX.Y.Z/...), built
# from this checkout by tools/release. Needs go, tar, gzip, curl and
# sha256sum/shasum.
#
#   tests/install_test.sh       # scratch files in .local/scratch/install-test
set -eu

cd "$(dirname "$0")/.."
ROOT=$(pwd)
INSTALLER=$ROOT/site/public/install.sh
VERSION=v9.8.7
# The scratch dir is a literal path below the repo root, removed before and after.
rm -rf .local/scratch/install-test
mkdir -p .local/scratch/install-test
T=$ROOT/.local/scratch/install-test
trap 'cd "$ROOT" && rm -rf .local/scratch/install-test' EXIT

fails=0
pass() { printf 'ok   %s\n' "$1"; }
fail() {
	printf 'FAIL %s\n' "$1"
	fails=$((fails + 1))
}

case $(uname -s) in Darwin) os=darwin ;; Linux) os=linux ;; *) echo "unsupported OS for this test" >&2; exit 2 ;; esac
case $(uname -m) in x86_64 | amd64) arch=amd64 ;; arm64 | aarch64) arch=arm64 ;; *) echo "unsupported arch" >&2; exit 2 ;; esac
ASSET=coderefinery_${VERSION#v}_${os}_${arch}.tar.gz

# sysbin: what the installer may use. GNU tar (Linux) runs gzip for -z.
mkdir -p "$T/sysbin"
for tool in sh uname tr sed awk mkdir cp chmod mv rm rmdir mktemp cat tar gzip curl sha256sum shasum; do
	if p=$(command -v "$tool" 2>/dev/null); then ln -s "$p" "$T/sysbin/$tool"; fi
done
if p=$(command -v perl 2>/dev/null); then ln -s "$p" "$T/sysbin/perl"; fi # shasum on macOS

# A release dir like the bucket: the real binary, SHA256SUMS, latest.json.
mkdir -p "$T/build" "$T/release/$VERSION"
(cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$T/build/coderefinery" ./cmd/coderefinery)
tar -czf "$T/release/$VERSION/$ASSET" -C "$T/build" coderefinery
if command -v sha256sum >/dev/null 2>&1; then
	(cd "$T/release/$VERSION" && sha256sum "$ASSET" >SHA256SUMS)
else
	(cd "$T/release/$VERSION" && shasum -a 256 "$ASSET" >SHA256SUMS)
fi
(cd "$ROOT" && go run ./tools/release manifest "$VERSION" "$T/release/$VERSION") >"$T/release/latest.json"

# run CASE [VAR=value ...]: pipe the installer into sh in a clean environment,
# from the case's own directory. Sets $out (stdout+stderr) and $code.
run() {
	name=$1
	shift
	home=$T/$name/home
	mkdir -p "$home" "$T/$name/tmp"
	envs="HOME=$home
TMPDIR=$T/$name/tmp
PATH=$T/sysbin"
	for kv in "$@"; do
		envs="$envs
$kv"
	done
	set +e
	out=$(
		IFS='
'
		# shellcheck disable=SC2086 # one assignment per line, split on purpose
		env -i $envs sh <"$INSTALLER" 2>&1
	)
	code=$?
	set -e
}

expect() { # expect CASE CODE PATTERN: exit code and an output line matching PATTERN
	if [ "$code" = "$2" ] && printf '%s\n' "$out" | grep -q -- "$3"; then
		pass "$1"
	else
		fail "$1: want exit $2 and output matching '$3', got exit $code:"
		printf '%s\n' "$out" | sed 's/^/     | /'
	fi
}

check() { # check NAME COMMAND...: a plain assertion
	name=$1
	shift
	if "$@"; then pass "$name"; else fail "$name"; fi
}

runs_as() { # runs_as BIN VERSION
	[ "$("$1" --version 2>/dev/null)" = "coderefinery $2" ]
}

empty_dir() { [ -z "$(ls -A "$1")" ]; }

# 1. Latest release into CODEREFINERY_INSTALL_DIR, which is not on PATH.
run latest "CODEREFINERY_DOWNLOAD_URL=file://$T/release/" "CODEREFINERY_INSTALL_DIR=$T/latest/bin"
expect "latest.json is read and the release installed" 0 "^coderefinery install: installed coderefinery $VERSION to $T/latest/bin/coderefinery\$"
expect "it downloads from the versioned directory" 0 "downloading file://$T/release/$VERSION/$ASSET\$"
expect "a PATH hint when the dir is not on PATH" 0 "export PATH=\"$T/latest/bin:\$PATH\""
check "the installed binary runs as $VERSION" runs_as "$T/latest/bin/coderefinery" "$VERSION"
check "the install dir holds only coderefinery" [ "$(ls -A "$T/latest/bin")" = coderefinery ]
check "the temporary directory is cleaned up" empty_dir "$T/latest/tmp"

# 2. Default dir ~/.local/bin, already on PATH: no hint.
run default "CODEREFINERY_DOWNLOAD_URL=file://$T/release" "PATH=$T/default/home/.local/bin:$T/sysbin"
expect "default install dir is ~/.local/bin" 0 "installed coderefinery $VERSION to $T/default/home/.local/bin/coderefinery\$"
if printf '%s\n' "$out" | grep -q 'is not on your PATH'; then fail "no PATH hint when the dir is on PATH"; else pass "no PATH hint when the dir is on PATH"; fi

# 3. A pinned version needs no latest.json; an existing binary is replaced.
mkdir -p "$T/pinned-release" "$T/pinned/bin"
cp -R "$T/release/$VERSION" "$T/pinned-release/"
printf '#!/bin/sh\necho old\n' >"$T/pinned/bin/coderefinery"
chmod +x "$T/pinned/bin/coderefinery"
run pinned "CODEREFINERY_DOWNLOAD_URL=file://$T/pinned-release" "CODEREFINERY_VERSION=$VERSION" "CODEREFINERY_INSTALL_DIR=$T/pinned/bin"
expect "CODEREFINERY_VERSION skips latest.json" 0 "installed coderefinery $VERSION to $T/pinned/bin/coderefinery\$"
check "an existing binary is replaced" runs_as "$T/pinned/bin/coderefinery" "$VERSION"

# 4. A checksum mismatch installs nothing.
mkdir -p "$T/bad-release/$VERSION"
cp "$T/release/latest.json" "$T/bad-release/"
cp "$T/release/$VERSION/$ASSET" "$T/bad-release/$VERSION/"
printf '%s  %s\n' 0000000000000000000000000000000000000000000000000000000000000000 "$ASSET" >"$T/bad-release/$VERSION/SHA256SUMS"
run badsum "CODEREFINERY_DOWNLOAD_URL=file://$T/bad-release" "CODEREFINERY_INSTALL_DIR=$T/badsum/bin"
expect "checksum mismatch is refused" 1 "checksum mismatch for $ASSET (SHA256SUMS: 0000000000000000000000000000000000000000000000000000000000000000, file: [0-9a-f]*); nothing installed\$"
check "nothing installed after a mismatch" [ ! -e "$T/badsum/bin" ]
check "the temporary directory is cleaned up after a failure" empty_dir "$T/badsum/tmp"

# 5. Unreachable release, malformed manifest, bad pinned version.
run nowhere "CODEREFINERY_DOWNLOAD_URL=file://$T/nowhere" "CODEREFINERY_INSTALL_DIR=$T/nowhere/bin"
expect "a missing latest.json names the URL" 1 "download failed: file://$T/nowhere/latest.json\$"
mkdir -p "$T/junk-release"
printf '{"assets":{}}\n' >"$T/junk-release/latest.json"
run junk "CODEREFINERY_DOWNLOAD_URL=file://$T/junk-release" "CODEREFINERY_INSTALL_DIR=$T/junk/bin"
expect "a manifest without a version is refused" 1 "not a release version: '' (want vX.Y.Z)\$"
run badpin "CODEREFINERY_DOWNLOAD_URL=file://$T/release" "CODEREFINERY_VERSION=latest" "CODEREFINERY_INSTALL_DIR=$T/badpin/bin"
expect "a pinned version must be vX.Y.Z" 1 "not a release version: 'latest' (want vX.Y.Z)\$"
run missing "CODEREFINERY_DOWNLOAD_URL=file://$T/release" "CODEREFINERY_VERSION=v9.9.9" "CODEREFINERY_INSTALL_DIR=$T/missing/bin"
expect "a version without a build fails on the archive" 1 "download failed: file://$T/release/v9.9.9/coderefinery_9.9.9_${os}_${arch}.tar.gz\$"

# 6. A compact one-line manifest is read too.
mkdir -p "$T/compact-release"
cp -R "$T/release/$VERSION" "$T/compact-release/"
printf '{"version":"%s","assets":{}}' "$VERSION" >"$T/compact-release/latest.json"
run compact "CODEREFINERY_DOWNLOAD_URL=file://$T/compact-release" "CODEREFINERY_INSTALL_DIR=$T/compact/bin"
expect "a one-line latest.json works" 0 "installed coderefinery $VERSION"

# 7. An install dir that cannot be written says what to do.
if [ "$(id -u)" != 0 ]; then
	mkdir -p "$T/ro/bin"
	chmod 555 "$T/ro/bin"
	run ro "CODEREFINERY_DOWNLOAD_URL=file://$T/release" "CODEREFINERY_INSTALL_DIR=$T/ro/bin"
	chmod 755 "$T/ro/bin"
	expect "a read-only install dir is reported" 1 "cannot write to $T/ro/bin; set CODEREFINERY_INSTALL_DIR to a directory you can write\$"
fi

# 8. One default download URL everywhere: installer, skill launcher, coderefinery update.
# shellcheck disable=SC2016 # the patterns match the scripts' literal ${...} text
installer_default=$(sed -n 's/^	base=${CODEREFINERY_DOWNLOAD_URL:-\(.*\)}$/\1/p' "$INSTALLER")
# shellcheck disable=SC2016
launcher_default=$(sed -n 's/^CODEREFINERY_DOWNLOAD_URL=${CODEREFINERY_DOWNLOAD_URL:-\(.*\)}$/\1/p' "$ROOT/skills/coderefinery/scripts/coderefinery")
go_default=$(sed -n 's/^	DefaultBaseURL *= "\(.*\)"$/\1/p' "$ROOT/internal/selfupdate/release.go")
if [ "$installer_default" = https://coderefinery.dev/dl ] && [ "$launcher_default" = "$installer_default" ] && [ "$go_default" = "$installer_default" ]; then
	pass "installer, launcher and update share the default URL"
else
	fail "default URLs differ: installer '$installer_default', launcher '$launcher_default', update '$go_default'"
fi

# Smoke: the installed binary does real work.
mkdir -p "$T/smoke"
printf 'def f(a):\n    return a + 1\n' >"$T/smoke/m.py"
(cd "$T/smoke" && env -i HOME="$T/smoke" PATH="$T/sysbin" "$T/latest/bin/coderefinery" dry --threshold 0.8 >"$T/smoke/out" 2>&1) && code=0 || code=$?
out=$(cat "$T/smoke/out")
expect "smoke: the installed coderefinery runs dry" 0 '^No duplicate candidates found\.$'

if [ "$fails" -ne 0 ]; then
	echo "$fails install test(s) failed"
	exit 1
fi
echo "all install tests passed"
