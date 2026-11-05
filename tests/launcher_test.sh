#!/bin/sh
# Tests for skills/coderefinery/scripts/coderefinery, the launcher the skill calls.
# Every case runs the launcher under `env -i` with a scratch HOME/cache and a
# PATH that holds only the tools that case allows, so nothing touches the real
# home directory or network. Needs go, tar, zip, curl and sha256sum/shasum.
#
#   tests/launcher_test.sh       # scratch files in .local/scratch/launcher-test
set -eu

cd "$(dirname "$0")/.."
ROOT=$(pwd)
LAUNCHER=$ROOT/skills/coderefinery/scripts/coderefinery
VERSION=$(sed -n 's/^CODEREFINERY_VERSION=//p' "$LAUNCHER")
# The scratch dir is a literal path below the repo root, removed before and after.
rm -rf .local/scratch/launcher-test
mkdir -p .local/scratch/launcher-test
T=$ROOT/.local/scratch/launcher-test
trap 'cd "$ROOT" && rm -rf .local/scratch/launcher-test' EXIT

fails=0
pass() { printf 'ok   %s\n' "$1"; }
fail() {
	printf 'FAIL %s\n' "$1"
	fails=$((fails + 1))
}

case $(uname -s) in Darwin) os=darwin ;; Linux) os=linux ;; *) echo "unsupported OS for this test" >&2; exit 2 ;; esac
case $(uname -m) in x86_64 | amd64) arch=amd64 ;; arm64 | aarch64) arch=arm64 ;; *) echo "unsupported arch" >&2; exit 2 ;; esac
ASSET=coderefinery_${VERSION#v}_${os}_${arch}.tar.gz
GOROOT_BIN=$(go env GOROOT)/bin

# sysbin: the base tools the launcher may use, without go or coderefinery.
# GNU tar (Linux) runs gzip for -z; bsdtar (macOS) does not need it.
mkdir -p "$T/sysbin"
for tool in sh uname mkdir rmdir rm mv chmod awk tar gzip curl sha256sum shasum cat; do
	if p=$(command -v "$tool" 2>/dev/null); then ln -s "$p" "$T/sysbin/$tool"; fi
done
# shasum is a perl script on macOS.
if p=$(command -v perl 2>/dev/null); then ln -s "$p" "$T/sysbin/perl"; fi

fake() { # fake PATH LABEL EXIT: a stand-in coderefinery that reports how it was reached
	# shellcheck disable=SC2016 # $* and CODEREFINERY_LAUNCHER expand in the fake, not here
	printf '#!/bin/sh\necho "%s:$*:${CODEREFINERY_LAUNCHER:-}"\nexit %s\n' "$2" "$3" >"$1"
	chmod +x "$1"
}

# run CASE [VAR=value ...] -- ARGS: run the launcher in a clean environment.
# Sets $out (stdout+stderr) and $code.
run() {
	name=$1
	shift
	home=$T/$name/home
	mkdir -p "$home"
	envs="HOME=$home"
	while [ "$1" != -- ]; do
		envs="$envs
$1"
		shift
	done
	shift
	set +e
	out=$(
		IFS='
'
		# shellcheck disable=SC2086 # one assignment per line, split on purpose
		env -i PATH="$T/sysbin" $envs "$LAUNCHER" "$@" 2>&1
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

# 1. CODEREFINERY_BIN wins over PATH and passes arguments and the exit status through.
mkdir -p "$T/bins" "$T/pathbin"
fake "$T/bins/explicit" explicit 3
fake "$T/pathbin/coderefinery" path 0
run explicit "CODEREFINERY_BIN=$T/bins/explicit" "PATH=$T/pathbin:$T/sysbin" -- crap --changed
expect "CODEREFINERY_BIN is used, args and exit status pass through" 3 '^explicit:crap --changed:$'

run explicit-missing "CODEREFINERY_BIN=$T/bins/nope" -- dry
expect "CODEREFINERY_BIN that is not executable fails with 127" 127 "CODEREFINERY_BIN=$T/bins/nope is not an executable file"

# 2. coderefinery on PATH.
run path "PATH=$T/pathbin:$T/sysbin" -- dry --threshold 0.8
expect "coderefinery on PATH is used" 0 '^path:dry --threshold 0.8:1$'

# 3. The cache, under XDG_CACHE_HOME and under HOME/.cache.
mkdir -p "$T/xdg/coderefinery/$VERSION"
fake "$T/xdg/coderefinery/$VERSION/coderefinery" xdg-cache 0
run xdg "XDG_CACHE_HOME=$T/xdg" -- mutate --scan
expect "cached binary under XDG_CACHE_HOME is used" 0 '^xdg-cache:mutate --scan:$'

mkdir -p "$T/homecache/home/.cache/coderefinery/$VERSION"
fake "$T/homecache/home/.cache/coderefinery/$VERSION/coderefinery" home-cache 0
run homecache -- crap
expect "cached binary under HOME/.cache is used" 0 '^home-cache:crap:$'

# 4. A launcher installed on PATH as "coderefinery" does not call itself forever.
mkdir -p "$T/selfbin"
ln -s "$LAUNCHER" "$T/selfbin/coderefinery"
run self "PATH=$T/selfbin:$T/sysbin" "XDG_CACHE_HOME=$T/xdg" -- dry
expect "launcher on PATH falls through to the cache" 0 '^xdg-cache:dry:1$'

# 5. Download: a local release dir laid out like the R2 bucket and the GitHub
# release (<version>/<archive>, <version>/SHA256SUMS), with the real binary.
# NOWHERE points both mirrors at nothing, so no case reaches the network.
NOWHERE="CODEREFINERY_DOWNLOAD_URL=file://$T/nowhere CODEREFINERY_RELEASE_URL=file://$T/nowhere"
mkdir -p "$T/build" "$T/release/$VERSION"
(cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$T/build/coderefinery" ./cmd/coderefinery)
tar -czf "$T/release/$VERSION/$ASSET" -C "$T/build" coderefinery
if command -v sha256sum >/dev/null 2>&1; then
	(cd "$T/release/$VERSION" && sha256sum "$ASSET" >SHA256SUMS)
else
	(cd "$T/release/$VERSION" && shasum -a 256 "$ASSET" >SHA256SUMS)
fi
# Junk left by an interrupted run is overwritten, not trusted.
mkdir -p "$T/dl-cache/coderefinery/$VERSION/.download"
echo junk >"$T/dl-cache/coderefinery/$VERSION/.download/$ASSET"
echo junk >"$T/dl-cache/coderefinery/$VERSION/.download/coderefinery"
run download "XDG_CACHE_HOME=$T/dl-cache" "CODEREFINERY_DOWNLOAD_URL=file://$T/release" "CODEREFINERY_RELEASE_URL=file://$T/nowhere" -- --version
expect "release asset is downloaded, verified and run" 0 "^coderefinery $VERSION\$"
expect "coderefinery.dev/dl (R2) is tried first" 0 "downloading file://$T/release/$VERSION/$ASSET\$"
if printf '%s\n' "$out" | grep -q nowhere; then fail "the GitHub mirror is not tried after R2 worked"; else pass "the GitHub mirror is not tried after R2 worked"; fi
leftover=
for f in "$T/dl-cache/coderefinery/$VERSION"/* "$T/dl-cache/coderefinery/$VERSION"/.[!.]*; do
	[ -e "$f" ] && [ "${f##*/}" != coderefinery ] && leftover="$leftover ${f##*/}"
done
if [ -x "$T/dl-cache/coderefinery/$VERSION/coderefinery" ] && [ -z "$leftover" ]; then
	pass "downloaded binary is cached and the temp dir is gone"
else
	fail "downloaded binary is cached and the temp dir is gone; left over:$leftover"
fi
# shellcheck disable=SC2086 # NOWHERE is two assignments
run download-again "XDG_CACHE_HOME=$T/dl-cache" $NOWHERE -- --help
expect "second run uses the cache without downloading" 0 '^Usage: coderefinery <command>'

# 6. A checksum mismatch installs nothing.
mkdir -p "$T/bad-release/$VERSION"
cp "$T/release/$VERSION/$ASSET" "$T/bad-release/$VERSION/"
printf '%s  %s\n' 0000000000000000000000000000000000000000000000000000000000000000 "$ASSET" >"$T/bad-release/$VERSION/SHA256SUMS"
run badsum "XDG_CACHE_HOME=$T/bad-cache" "CODEREFINERY_DOWNLOAD_URL=file://$T/bad-release" "CODEREFINERY_RELEASE_URL=file://$T/bad-release" -- --version
expect "checksum mismatch is refused" 127 "checksum mismatch for $ASSET"
if [ ! -e "$T/bad-cache/coderefinery/$VERSION/coderefinery" ]; then pass "nothing cached after a mismatch"; else fail "nothing cached after a mismatch"; fi
expect "the failure message says how to fix it" 127 'set CODEREFINERY_BIN to its path'

# 6b. R2 unreachable: the GitHub release is the fallback.
run fallback "XDG_CACHE_HOME=$T/fallback-cache" "CODEREFINERY_DOWNLOAD_URL=file://$T/nowhere" "CODEREFINERY_RELEASE_URL=file://$T/release" -- --version
expect "the GitHub release is used when R2 fails" 0 "^coderefinery $VERSION\$"
expect "after R2 was tried first" 0 "download failed: file://$T/nowhere/$VERSION/$ASSET\$"

# 7. go install fallback, from a file:// module proxy holding the root module
# (its Go sources) at the plain vX.Y.Z tag, as the real proxy serves it.
modpath=github.com/matheusAle/coderefinery
proxy=$T/proxy/github.com/matheus!ale/coderefinery/@v
mkdir -p "$proxy" "$T/modsrc/$modpath@$VERSION"
# Like a real module zip, leave out nested modules (dirs with their own go.mod,
# e.g. Go testdata projects).
(cd "$ROOT" && nested=$(find cmd internal tools testdata -name go.mod -exec dirname {} \;) &&
find go.mod cmd internal tools testdata -type f -print | while read -r f; do
	skip=
	for d in $nested; do case "$f" in "$d"/*) skip=1 ;; esac; done
	[ -n "$skip" ] && continue
	mkdir -p "$T/modsrc/$modpath@$VERSION/$(dirname "$f")"
	cp "$f" "$T/modsrc/$modpath@$VERSION/$f"
done)
(cd "$T/modsrc" && zip -qr "$proxy/$VERSION.zip" "$modpath@$VERSION")
cp "$ROOT/go.mod" "$proxy/$VERSION.mod"
printf '{"Version":"%s"}\n' "$VERSION" >"$proxy/$VERSION.info"
printf '%s\n' "$VERSION" >"$proxy/list"
# shellcheck disable=SC2086 # NOWHERE is two assignments
run goinstall "PATH=$T/sysbin:$GOROOT_BIN" "XDG_CACHE_HOME=$T/go-cache" $NOWHERE \
	"GOPROXY=file://$T/proxy" GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-modcacherw \
	"GOPATH=$T/gopath" "GOCACHE=$T/gocache" -- --version
expect "go install fallback builds the pinned version" 0 "^coderefinery $VERSION\$"
expect "go install runs only after the download failed" 0 'download failed: file://'

# 8. Nothing works: no release, no go.
# shellcheck disable=SC2086 # NOWHERE is two assignments
run nothing "XDG_CACHE_HOME=$T/none-cache" $NOWHERE -- crap
expect "no binary at all exits 127" 127 'go is not installed'
expect "and explains the options" 127 "could not install coderefinery $VERSION"

# Smoke: the real launcher end to end, cached binary, a real subcommand.
mkdir -p "$T/smoke"
printf 'def f(a):\n    return a + 1\n' >"$T/smoke/m.py"
(cd "$T/smoke" && env -i HOME="$T/smoke" PATH="$T/sysbin" XDG_CACHE_HOME="$T/dl-cache" "$LAUNCHER" dry --threshold 0.8 >"$T/smoke/out" 2>&1) && code=0 || code=$?
out=$(cat "$T/smoke/out")
expect "smoke: dry runs through the launcher" 0 '^No duplicate candidates found\.$'
if [ "$(cat "$T/smoke/.metrics/dry.edn")" = '{:candidates []}' ]; then pass "smoke: .metrics/dry.edn written"; else fail "smoke: .metrics/dry.edn written"; fi

if [ "$fails" -ne 0 ]; then
	echo "$fails launcher test(s) failed"
	exit 1
fi
echo "all launcher tests passed"
