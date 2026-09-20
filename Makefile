.PHONY: all build test lint sbom clean

all: build test

build:
	go build -o bin/hive ./cmd/hive
	go build -o bin/hive-node ./node/

test:
	go test ./... -race -timeout 60s -coverprofile=coverage.out

lint:
	golangci-lint run ./...

sbom:
	syft dir:. -o spdx-json > sbom.json

clean:
	rm -rf bin/ coverage.out sbom.json
