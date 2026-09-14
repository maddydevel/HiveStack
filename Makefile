.PHONY: all build test lint clean

all: build

build:
	go build -o bin/hive ./cmd/hive
	go build -o bin/hive-node ./node/

test:
	go test ./... -v -race -cover

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
