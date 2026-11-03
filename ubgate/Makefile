# ubgate build. Source ../harness/env.sh first (or let the targets do it) so
# every Go cache stays inside the ub-mode folder.
SHELL := /bin/bash
ENV := source ../harness/env.sh &&
PY_CORPUS ?= testdata/python ../fixtures/python ../upstream/crapper/src ../upstream/crapper/tests ../upstream/dryer/src ../upstream/mutator/src

.PHONY: build test vet ci treediff dist clean

build:
	$(ENV) go build -o bin/ubgate ./cmd/ubgate

vet:
	$(ENV) gofmt -l . | (! grep .) && go vet ./...

test:
	$(ENV) go test ./...

ci: vet test build

treediff:
	$(ENV) go run ./tools/treediff -q $(PY_CORPUS)

# Release archives and SHA256SUMS in ../dist, named as the skill's launcher
# (skills/ub-mode/scripts/ubgate) downloads them. make dist VERSION=v1.2.3
# No Windows build: internal/mutate uses Unix process groups (syscall.Kill).
RELEASE_TARGETS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64
dist:
	@test -n "$(VERSION)" || { echo "usage: make dist VERSION=v1.2.3" >&2; exit 2; }
	$(ENV) set -euo pipefail; mkdir -p ../dist; \
	for t in $(RELEASE_TARGETS); do \
	  name=ubgate_$(VERSION:v%=%)_$${t%/*}_$${t#*/}; \
	  mkdir -p ../dist/$$name; \
	  CGO_ENABLED=0 GOOS=$${t%/*} GOARCH=$${t#*/} go build -trimpath \
	    -ldflags "-s -w -X main.version=$(VERSION)" -o ../dist/$$name/ubgate ./cmd/ubgate; \
	  tar -czf ../dist/$$name.tar.gz -C ../dist/$$name ubgate; \
	done; \
	cd ../dist && shasum -a 256 ubgate_$(VERSION:v%=%)_*.tar.gz > SHA256SUMS

clean:
	rm -rf bin ../dist
