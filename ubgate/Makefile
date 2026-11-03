# ubgate build. Source ../harness/env.sh first (or let the targets do it) so
# every Go cache stays inside the ub-mode folder.
SHELL := /bin/bash
ENV := source ../harness/env.sh &&
PY_CORPUS ?= testdata/python ../fixtures/python ../upstream/crapper/src ../upstream/crapper/tests ../upstream/dryer/src ../upstream/mutator/src

.PHONY: build test vet ci treediff clean

build:
	$(ENV) go build -o bin/ubgate ./cmd/ubgate

vet:
	$(ENV) gofmt -l . | (! grep .) && go vet ./...

test:
	$(ENV) go test ./...

ci: vet test build

treediff:
	$(ENV) go run ./tools/treediff -q $(PY_CORPUS)

clean:
	rm -rf bin
