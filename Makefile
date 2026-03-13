BINARY  := protoc-gen-go-echo
GOBIN   := $(shell go env GOPATH)/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "devel")
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install test vet lint clean

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) .

install:
	go build -ldflags '$(LDFLAGS)' -o $(GOBIN)/$(BINARY) .

test:
	go test -race ./...

vet:
	go vet ./...

lint: vet
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint 未安装，跳过（go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest）"

clean:
	rm -f $(BINARY)
