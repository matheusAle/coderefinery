#!/bin/sh
# Installs coderefinery: curl -fsSL https://coderefinery.dev/install.sh | sh
#
# Downloads the release archive for this OS and CPU, checks it against the
# release's SHA256SUMS, and puts the coderefinery binary in ~/.local/bin.
# Afterwards `coderefinery update` keeps it current (and also checks the
# signature of SHA256SUMS). Settings, all optional:
#
#   CODEREFINERY_VERSION=v1.2.3      install this version, not the latest
#   CODEREFINERY_INSTALL_DIR=<dir>   install here, not ~/.local/bin
#   CODEREFINERY_DOWNLOAD_URL=<url>  release base URL (a mirror), not https://coderefinery.dev/dl
#
#   curl -fsSL https://coderefinery.dev/install.sh | CODEREFINERY_INSTALL_DIR=/usr/local/bin sh
#
# Release builds: macOS and Linux, amd64 and arm64. Source:
# https://github.com/matheusAle/coderefinery (site/public/install.sh).
set -eu

say() { printf 'coderefinery install: %s\n' "$*"; }
die() {
	printf 'coderefinery install: %s\n' "$*" >&2
	exit 1
}

fetch() { # fetch URL FILE
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 2 -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		die "neither curl nor wget is installed"
	fi
}

sha256_of() { # sha256_of FILE -> hex digest on stdout
	if command -v sha256sum >/dev/null 2>&1; then
		sum=$(sha256sum "$1")
	elif command -v shasum >/dev/null 2>&1; then
		sum=$(shasum -a 256 "$1")
	else
		die "neither sha256sum nor shasum is installed"
	fi
	printf '%s\n' "${sum%% *}"
}

# Everything runs from main, so a download cut short never runs half a script.
main() {
	case $(uname -s) in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) die "no release build for $(uname -s); build from source: https://github.com/matheusAle/coderefinery#install-coderefinery" ;;
	esac
	case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) die "no release build for $(uname -m); build from source: https://github.com/matheusAle/coderefinery#install-coderefinery" ;;
	esac
	command -v tar >/dev/null 2>&1 || die "tar is not installed"

	base=${CODEREFINERY_DOWNLOAD_URL:-https://coderefinery.dev/dl}
	base=${base%/}
	if [ -n "${CODEREFINERY_INSTALL_DIR:-}" ]; then
		dir=$CODEREFINERY_INSTALL_DIR
	else
		[ -n "${HOME:-}" ] || die "set HOME or CODEREFINERY_INSTALL_DIR"
		dir=$HOME/.local/bin
	fi

	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t coderefinery) || die "cannot create a temporary directory"
	# Only the files named here are ever deleted, then the (empty) directory.
	trap 'rm -f "$tmp/latest.json" "$tmp/SHA256SUMS" "$tmp/archive.tar.gz" "$tmp/coderefinery"; rmdir "$tmp" 2>/dev/null || true' EXIT
	trap 'exit 1' HUP INT TERM

	version=${CODEREFINERY_VERSION:-}
	if [ -z "$version" ]; then
		fetch "$base/latest.json" "$tmp/latest.json" || die "download failed: $base/latest.json"
		version=$(tr -d '\n\r' <"$tmp/latest.json" | sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
	fi
	case $version in
	v[0-9]*.[0-9]*.[0-9]*) ;;
	*) die "not a release version: '${version}' (want vX.Y.Z)" ;;
	esac

	asset=coderefinery_${version#v}_${os}_${arch}.tar.gz
	url=$base/$version
	say "downloading $url/$asset"
	fetch "$url/$asset" "$tmp/archive.tar.gz" || die "download failed: $url/$asset"
	fetch "$url/SHA256SUMS" "$tmp/SHA256SUMS" || die "download failed: $url/SHA256SUMS"
	want=$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/SHA256SUMS")
	got=$(sha256_of "$tmp/archive.tar.gz")
	if [ -z "$want" ] || [ "$want" != "$got" ]; then
		die "checksum mismatch for $asset (SHA256SUMS: ${want:-missing}, file: $got); nothing installed"
	fi
	tar -xzf "$tmp/archive.tar.gz" -C "$tmp" coderefinery 2>/dev/null || die "$asset has no coderefinery binary"

	mkdir -p "$dir" 2>/dev/null || die "cannot create $dir; set CODEREFINERY_INSTALL_DIR to a directory you can write"
	# Copy next to the target, then rename: a running coderefinery is never half-written.
	cp "$tmp/coderefinery" "$dir/.coderefinery.new" 2>/dev/null ||
		die "cannot write to $dir; set CODEREFINERY_INSTALL_DIR to a directory you can write"
	chmod 755 "$dir/.coderefinery.new"
	mv -f "$dir/.coderefinery.new" "$dir/coderefinery"
	installed=$("$dir/coderefinery" --version) || die "$dir/coderefinery does not run on this machine"
	say "installed $installed to $dir/coderefinery"

	case :${PATH:-}: in
	*:"$dir":*) ;;
	*)
		say "$dir is not on your PATH. Add it, e.g. to ~/.profile, ~/.bashrc or ~/.zshrc:"
		# shellcheck disable=SC2016 # $PATH is printed literally, for the user's rc file
		printf '\n    export PATH="%s:$PATH"\n\n' "$dir"
		;;
	esac
	say "update later with: coderefinery update"
}

main "$@"
