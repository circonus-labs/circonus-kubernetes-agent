BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
BRANCH_PRIMARY=$(shell git symbolic-ref refs/remotes/origin/HEAD | sed 's;^refs/remotes/origin/;;')
BUILD_FLAGS=-mod=vendor
GCI=$(shell which gci)
GIT=$(shell which git)
GITCOMM=$(shell which gitcomm)
GO=$(shell which go)
GOFUMPT=$(shell which gofumpt)
GOLANGCI_LINT=$(shell which golangci-lint)
GORELEASER=$(shell which goreleaser)
NOVENDOR=$(shell go list -f {{.Dir}} ./...)
PODMAN=$(shell (which podman &>/dev/null && which podman) || which docker)
SCRIPTS_DIR=./.scripts
TRUFFLEHOG=$(shell which trufflehog)

SHELL=/usr/bin/env bash

.PHONY: gofumpt gci go_mod golangci_lint go_test trufflehog render build_deps changelog build commit push release is_primary tag 

gofumpt:
	$(GOFUMPT) -l -w $(NOVENDOR)

gci:
	$(GCI) write --skip-generated -s standard,default $(NOVENDOR)

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

build_deps: render gofumpt gci go_mod golangci_lint go_test trufflehog

changelog:
	$(SCRIPTS_DIR)/changelog.sh

build: build_deps
	DOCKER_REGISTRY=$(DOCKER_REGISTRY) $(GORELEASER) --rm-dist --snapshot

commit: build_deps
	$(GIT) status
	$(GITCOMM)

commit_for_tag: changelog build_deps
	$(GIT) status
	$(GITCOMM)

push:
	$(GIT) push -u origin $(BRANCH)

is_primary:
	[ "$(BRANCH)" = "$(BRANCH_PRIMARY)" ] || ( echo "Current branch is not repo primary branch" && exit 1 )

tag: is_primary build_deps
	$(GIT) tag -s

release: is_primary build_deps
	DOCKER_REGISTRY=registry.hub.docker.com/circonus $(GORELEASER) release --rm-dist

