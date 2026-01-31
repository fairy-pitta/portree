APP_NAME := portree
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/fairy-pitta/portree/cmd.version=$(VERSION) -X github.com/fairy-pitta/portree/cmd.commit=$(COMMIT) -X github.com/fairy-pitta/portree/cmd.date=$(DATE)"

.PHONY: build test lint clean install

build:
	go build $(LDFLAGS) -o $(APP_NAME) .

install:
	go install $(LDFLAGS) .

test:
	go test ./... -race -count=1 -v

test-short:
	go test ./... -short -count=1

lint:
	golangci-lint run ./...

clean:
	rm -f $(APP_NAME)
	rm -rf dist/

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

all: fmt vet lint test build
