GO ?= go
BIN_DIR ?= bin
SERVER_ADDR ?= :8080
SERVER_ROOT ?= data
PREFIX ?= /usr/local
DESTDIR ?=
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || true)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true)

ifeq ($(OS),Windows_NT)
    EXE_SUFFIX := .exe
else
    EXE_SUFFIX :=
endif

SERVER_BIN := $(BIN_DIR)/ghh-server$(EXE_SUFFIX)
CLIENT_BIN := $(BIN_DIR)/ghh$(EXE_SUFFIX)

# Static build flags
STATIC_FLAGS := CGO_ENABLED=0
LDVARS := -s -w
ifneq ($(strip $(VERSION)),)
    LDVARS += -X github-hub/internal/version.Version=$(VERSION)
endif
ifneq ($(strip $(COMMIT)),)
    LDVARS += -X github-hub/internal/version.Commit=$(COMMIT)
endif
ifneq ($(strip $(BUILD_DATE)),)
    LDVARS += -X github-hub/internal/version.BuildDate=$(BUILD_DATE)
endif
LDFLAGS := -ldflags '$(strip $(LDVARS))'

.PHONY: all build build-server build-client build-static build-server-static build-client-static run run-server test test-json test-unit vet fmt clean install uninstall
.PHONY: build-cross

all: build

build: build-server build-client

build-server:
	$(GO) build -o $(SERVER_BIN) ./cmd/ghh-server

build-client:
	$(GO) build -o $(CLIENT_BIN) ./cmd/ghh

build-static: build-server-static build-client-static

build-server-static:
	$(STATIC_FLAGS) $(GO) build -trimpath $(LDFLAGS) -o $(SERVER_BIN) ./cmd/ghh-server

build-client-static:
	$(STATIC_FLAGS) $(GO) build -trimpath $(LDFLAGS) -o $(CLIENT_BIN) ./cmd/ghh

define build_pair
	@echo "Building $(1)-$(2) ..."
	@mkdir -p $(BIN_DIR)/$(1)-$(2)
	@GOOS=$(1) GOARCH=$(2) CGO_ENABLED=0 $(GO) build -trimpath $(LDFLAGS) -o $(BIN_DIR)/$(1)-$(2)/ghh$(3) ./cmd/ghh
	@GOOS=$(1) GOARCH=$(2) CGO_ENABLED=0 $(GO) build -trimpath $(LDFLAGS) -o $(BIN_DIR)/$(1)-$(2)/ghh-server$(3) ./cmd/ghh-server
endef

# Cross build for major platforms/arches with clear bin separation
build-cross:
	$(call build_pair,linux,amd64,)
	$(call build_pair,linux,arm64,)
	$(call build_pair,darwin,amd64,)
	$(call build_pair,darwin,arm64,)
	$(call build_pair,windows,amd64,.exe)
	$(call build_pair,windows,arm64,.exe)

run: run-server

run-server: build-server
	$(SERVER_BIN) --addr $(SERVER_ADDR) --root $(SERVER_ROOT)

test:
	$(GO) test ./... -race -cover

# Run tests with JSON output to test_result.json (parseable, no TTY needed)
test-json:
	$(GO) test ./... -json -count=1 -timeout 60s 2>&1 | tee test_result.jsonl

# Run unit tests only (storage, server, client), faster
test-unit:
	$(GO) test ./internal/storage ./internal/server ./internal/client -v -count=1 -timeout 30s

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf $(BIN_DIR)

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(SERVER_BIN) $(DESTDIR)$(PREFIX)/bin/
	install -m 755 $(CLIENT_BIN) $(DESTDIR)$(PREFIX)/bin/

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/ghh-server$(EXE_SUFFIX)
	rm -f $(DESTDIR)$(PREFIX)/bin/ghh$(EXE_SUFFIX)
