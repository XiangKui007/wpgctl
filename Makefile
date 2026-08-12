# wpgctl Makefile
# 用法：make build / make build-linux / make ui-build / make release

APP      := wpgctl
MODULE   := github.com/wpg/wpgctl
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.GitCommit=$(COMMIT) -X $(MODULE)/internal/version.BuildTime=$(BUILT)

.PHONY: all build build-linux build-arm64 test fmt vet tidy ui-build release

all: build

## 前端构建（嵌入 internal/ui/dist）
ui-build:
	cd web && npm install && npm run build

## 本机编译
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP)$(shell go env GOEXE) ./cmd/wpgctl

## Linux amd64（现场主架构）
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-linux-amd64 ./cmd/wpgctl

## Linux arm64（国产 ARM）
build-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/$(APP)-linux-arm64 ./cmd/wpgctl

## 完整发布：先编前端再交叉编译
release: ui-build build-linux build-arm64

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy
