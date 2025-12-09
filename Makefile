GO ?= go
BIN_DIR ?= bin

ifeq ($(OS),Windows_NT)
    EXE_SUFFIX := .exe
else
    EXE_SUFFIX :=
endif

SERVER_BIN := $(BIN_DIR)/ghh-server$(EXE_SUFFIX)
CLIENT_BIN := $(BIN_DIR)/ghh$(EXE_SUFFIX)

.PHONY: all build build-server build-client test vet fmt

all: build

build: build-server build-client

build-server:
	$(GO) build -o $(SERVER_BIN) ./cmd/ghh-server

build-client:
	$(GO) build -o $(CLIENT_BIN) ./cmd/ghh

test:
	$(GO) test ./... -race -cover

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...
