.PHONY: build test test-integration lint clean build-all

BINARY := mm-ready
VERSION := 0.1.0

build:
	go build -o bin/$(BINARY) .

test:
	go test ./...

test-integration:
	go test -tags integration ./tests/ -v

lint:
	go vet ./...

clean:
	rm -rf bin/

build-all:
	GOOS=linux   GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -o bin/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -o bin/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -o bin/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY)-windows-amd64.exe .
