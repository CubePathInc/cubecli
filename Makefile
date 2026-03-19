VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/CubePathInc/cubecli/internal/version.Version=$(VERSION) \
	-X github.com/CubePathInc/cubecli/internal/version.Commit=$(COMMIT) \
	-X github.com/CubePathInc/cubecli/internal/version.Date=$(DATE)

.PHONY: build clean test lint install

build:
	go build -ldflags "$(LDFLAGS)" -o cubecli .

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -f cubecli

test:
	go test ./...

lint:
	golangci-lint run

snapshot:
	goreleaser release --snapshot --clean
