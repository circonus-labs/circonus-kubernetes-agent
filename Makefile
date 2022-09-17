BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
BRANCH_PRIMARY=$(shell git symbolic-ref refs/remotes/origin/HEAD | sed 's;^refs/remotes/origin/;;')
BUILD_FLAGS=-mod=vendor
GO=$(shell which go)
GOFMT=$(shell which gofmt)
GOLANGCI_LINT=$(shell which golangci-lint)
GORELEASER=$(shell which goreleaser)
NOVENDOR=$(shell go list -f {{.Dir}} ./...)
PODMAN=$(shell (which podman &>/dev/null && which podman) || which docker)
SCRIPTS_DIR=./.scripts
TRUFFLEHOG=$(shell which trufflehog)

SHELL=/usr/bin/env bash

.PHONY: gofmt go_mod golangci_lint go_test trufflehog render build_deps changelog build commit push release is_primary tag 

gofmt:
	$(GOFMT) -s -w $(NOVENDOR)

go_mod:
	$(GO) mod tidy
	$(GO) mod vendor

golangci_lint:
	$(GOLANGCI_LINT) run

go_test:
	$(GO) test $(BUILD_FLAGS) -v -race -cover ./...

trufflehog:
	$(TRUFFLEHOG) git file://./

render:
	$(SCRIPTS_DIR)/render.sh

build_deps: render gofmt go_mod golangci_lint go_test trufflehog

changelog:
	$(SCRIPTS_DIR)/changelog.sh

build: build_deps
	$(GORELEASER) --rm-dist --snapshot

commit: build_deps
	$(GITCOMM)

commit_for_tag: changelog build_deps
	$(GITCOMM)

push:
	git push -u origin $(BRANCH)

is_primary:
	[ "$(BRANCH)" = "$(BRANCH_PRIMARY)" ] || ( echo "Current branch is not repo primary branch" && exit 1 )

tag: is_primary build_deps
	git tag -s

release: is_primary build_deps
	$(GORELEASER) release --rm-dist

