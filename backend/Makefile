# Backend developer entry points. `make ci` is what the pipeline runs (O0-03 extends it).
GO ?= go
COVER_MIN ?= 80

.PHONY: build lint test test-integration cover ci run

build:
	CGO_ENABLED=0 $(GO) build -o bin/server ./cmd/server

lint:
	golangci-lint run

test:
	$(GO) test -race ./...

test-integration:
	$(GO) test -race -tags integration ./...

cover:
	$(GO) test -coverpkg=./... -coverprofile=cover.out ./...
	@$(GO) tool cover -func=cover.out | awk -v min=$(COVER_MIN) '/^total:/ { sub("%","",$$3); print "total coverage " $$3 "%"; if ($$3+0 < min) { print "below " min "%"; exit 1 } }'

ci: lint test cover

run:
	$(GO) run ./cmd/server
