BINARY_DIR   := bin
VORND        := $(BINARY_DIR)/vornd
VORNCTL      := $(BINARY_DIR)/vornctl
GPU_HEALTHD  := $(BINARY_DIR)/vorn-gpu-healthd
MODEL_UNLOCK := $(BINARY_DIR)/vorn-model-unlocker
GO           := go
GOFLAGS      := -trimpath
LDFLAGS      := -s -w -X github.com/vornos/buildos/pkg/config.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PLATFORMS    := linux/amd64
PROTO_DIR    := pkg/api/proto

.PHONY: all build test lint fmt vet clean image run-dev install proto

all: build

## ── Build ──────────────────────────────────────────────

build: build-vornd build-vornctl build-gpu-healthd build-model-unlocker

build-vornd:
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(VORND) ./cmd/vornd

build-vornctl:
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(VORNCTL) ./cmd/vornctl

build-gpu-healthd:
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(GPU_HEALTHD) ./cmd/vorn-gpu-healthd

build-model-unlocker:
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(MODEL_UNLOCK) ./cmd/vorn-model-unlocker

## ── Protobuf ─────────────────────────────────────────

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       $(PROTO_DIR)/vorn.proto

## ── Quality ────────────────────────────────────────────

test:
	$(GO) test -race -count=1 ./...

lint:
	golangci-lint run ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

## ── Dev ────────────────────────────────────────────────

run-dev: build-vornd
	$(VORND) --config configs/vornd.yaml

## ── Install ────────────────────────────────────────────

install: build
	install -m 0755 $(VORND)        /opt/vorn/bin/vornd
	install -m 0755 $(VORNCTL)      /opt/vorn/bin/vornctl
	install -m 0755 $(GPU_HEALTHD)  /opt/vorn/bin/vorn-gpu-healthd
	install -m 0755 $(MODEL_UNLOCK) /opt/vorn/bin/vorn-model-unlocker
	ln -sf /opt/vorn/bin/vornd   /usr/local/bin/vornd
	ln -sf /opt/vorn/bin/vornctl /usr/local/bin/vornctl

## ── Image ──────────────────────────────────────────────

image: build
	@echo "Building Vorn OS base image (requires packer)..."
	@mkdir -p deploy/packer/files
	cp $(BINARY_DIR)/* deploy/packer/files/
	cd deploy/packer && packer build vornos.pkr.hcl

## ── Clean ──────────────────────────────────────────────

clean:
	rm -rf $(BINARY_DIR)
	rm -rf deploy/packer/files
