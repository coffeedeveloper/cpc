.PHONY: build install test clean
VERSION ?= dev
build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/cpc ./cmd/cpc
install:
	go install -trimpath -ldflags "-s -w -X main.version=$(VERSION)" ./cmd/cpc
test:
	go test -race ./...
	go vet ./...
clean:
	rm -rf bin dist
