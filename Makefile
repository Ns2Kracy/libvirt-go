SHELL := /usr/bin/env bash
GO ?= go
GO_TEST_FLAGS ?= -count=1
GO_FILES := $(shell find . -type f -name '*.go' -not -path './.git/*')

.PHONY: all build check-format check-generated check-tidy ci coverage cross \
	test test-integration test-race test-real vet

all: ci

build:
	CGO_ENABLED=0 $(GO) build ./...

check-format:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [[ -n "$$unformatted" ]]; then \
		echo "Go files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

check-generated:
	$(GO) generate ./...
	git diff --exit-code -- libvirt_api.gen.go

check-tidy:
	$(GO) mod tidy
	git diff --exit-code -- go.mod go.sum

vet:
	CGO_ENABLED=0 $(GO) vet ./...

test:
	CGO_ENABLED=0 $(GO) test $(GO_TEST_FLAGS) ./...

test-race:
	$(GO) test -race $(GO_TEST_FLAGS) ./...

test-integration:
	LIBVIRT_INTEGRATION=1 CGO_ENABLED=0 $(GO) test $(GO_TEST_FLAGS) -v ./integration

test-real:
	@test "$${LIBVIRT_REAL_INTEGRATION:-}" = 1
	@test "$${LIBVIRT_REAL_ALLOW_MUTATION:-}" = 1
	@test -n "$${LIBVIRT_REAL_URI:-}"
	CGO_ENABLED=0 $(GO) test $(GO_TEST_FLAGS) -run RealIntegration -v ./integration

cross:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build ./...

coverage:
	CGO_ENABLED=0 $(GO) test $(GO_TEST_FLAGS) -covermode=atomic \
		-coverprofile=coverage.out ./...

ci: check-format check-generated check-tidy build vet test test-race cross
