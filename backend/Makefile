# Backend developer entry points. `make ci` is what the GitHub Actions workflow runs (.github/workflows/ci.yml).
GO ?= go
COVER_MIN ?= 80

.PHONY: build lint test test-integration cover ci run generate generate-check spec-lint

build:
	CGO_ENABLED=0 $(GO) build -o bin/server ./cmd/server

lint:
	golangci-lint run

test:
	$(GO) test -race ./...

test-integration:
	$(GO) test -race -tags integration ./...

# One run: unit + integration (needs Docker), -race, a single merged profile.
cover:
	$(GO) test -race -tags integration -coverpkg=./... -coverprofile=cover.out ./...
	@# ADR-0001 O2 (recommended, awaiting confirmation): generated code is outside the coverage denominator.
	@grep -v '\.gen\.go:' cover.out > cover.out.tmp && mv -f cover.out.tmp cover.out
	@$(GO) tool cover -func=cover.out | awk -v min=$(COVER_MIN) '/^total:/ { sub("%","",$$3); print "total coverage " $$3 "%"; if ($$3+0 < min) { print "below " min "%"; exit 1 } }'

generate:
	$(GO) generate ./...

# Generated code is committed; fail when it is stale (B0-12 acceptance: `go generate` no diff).
generate-check: generate
	@git diff --exit-code -- internal/api || { echo "generated code is stale: run make generate"; exit 1; }

spec-lint:
	cd api && npx --yes @stoplight/spectral-cli@6.15.0 lint openapi.yaml --ruleset .spectral.yaml --fail-severity=warn

ci: lint generate-check spec-lint cover

run:
	$(GO) run ./cmd/server
