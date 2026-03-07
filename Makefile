.PHONY: build test lint vet ci

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

lint: vet

ci: build vet test
