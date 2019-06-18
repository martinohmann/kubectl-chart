.DEFAULT_GOAL := help

PKG_NAME := github.com/martinohmann/kubectl-chart

.PHONY: help
help:
	@grep -E '^[a-zA-Z-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "[32m%-12s[0m %s\n", $$1, $$2}'

.PHONY: deps
deps: ## install go deps
	go mod vendor

.PHONY: build
build: ## build kubectl-chart
	go build \
		-ldflags "-s -w \
			-X $(PKG_NAME)/pkg/version.gitVersion=$$(git describe --tags 2>/dev/null || echo v0.0.0-master) \
			-X $(PKG_NAME)/pkg/version.gitCommit=$$(git rev-parse HEAD) \
			-X $(PKG_NAME)/pkg/version.buildDate=$$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
		./cmd/*

.PHONY: install
install: build ## install kubectl-chart
	cp kubectl-chart $(GOPATH)/bin/kubectl-chart

.PHONY: test
test: ## run tests
	go test -race -tags="$(TAGS)" $$(go list ./... | grep -v /vendor/)

.PHONY: vet
vet: ## run go vet
	go vet $$(go list ./... | grep -v /vendor/)

.PHONY: coverage
coverage: ## generate code coverage
	scripts/coverage

.PHONY: misspell
misspell: ## check spelling in go files
	misspell *.go

.PHONY: lint
lint: ## lint go files
	golint ./...
