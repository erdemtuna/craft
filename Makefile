.PHONY: build test vet lint ci

build:
	go build ./cmd/craft

test:
	go test -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

ci: vet lint test build
