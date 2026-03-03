GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	Git_Bash=$(subst \,/,$(subst cmd\,bindiff\git-bash.exe,$(dir $(shell where git))))
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
else
	API_PROTO_FILES=$(shell find api -name "*.proto")
endif

# App-specific conf proto files
USER_CONF_PROTO=app/user/internal/conf/conf.proto
FILE_CONF_PROTO=app/file/internal/conf/conf.proto

.PHONY: init
# Install protoc plugins and tools
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: api
# Generate API proto (gRPC only, no HTTP)
api:
	protoc --proto_path=./api \
		   --proto_path=./third_party \
		   --go_out=paths=source_relative:./api \
		   --go-grpc_out=paths=source_relative:./api \
		   $(API_PROTO_FILES)

.PHONY: conf
# Generate conf proto for all services
conf: conf-user conf-file

.PHONY: conf-user
# Generate conf proto for user service
conf-user:
	protoc --proto_path=./app/user/internal/conf \
		   --proto_path=./third_party \
		   --go_out=paths=source_relative:./app/user/internal/conf \
		   $(USER_CONF_PROTO)

.PHONY: conf-file
# Generate conf proto for file service
conf-file:
	protoc --proto_path=./app/file/internal/conf \
		   --proto_path=./third_party \
		   --go_out=paths=source_relative:./app/file/internal/conf \
		   $(FILE_CONF_PROTO)

.PHONY: wire
# Generate wire injection for all services
wire: wire-user wire-file

.PHONY: wire-user
# Generate wire injection for user service
wire-user:
	cd app/user/cmd && wire

.PHONY: wire-file
# Generate wire injection for file service
wire-file:
	cd app/file/cmd && wire

.PHONY: build
# Build all services
build: build-user build-file build-gateway

.PHONY: build-user
# Build user service
build-user:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/user-service ./app/user/cmd

.PHONY: build-file
# Build file service
build-file:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/file-service ./app/file/cmd

.PHONY: build-gateway
# Build gateway
build-gateway:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/gateway ./app/gateway/cmd

.PHONY: generate
generate:
	go generate ./...

.PHONY: test
# Run all tests
test:
	go test -v -cover ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: run-user
# Run user service locally
run-user:
	go run ./app/user/cmd -conf ./app/user/configs/

.PHONY: run-file
# Run file service locally
run-file:
	go run ./app/file/cmd -conf ./app/file/configs/

.PHONY: run-gateway
# Run gateway locally
run-gateway:
	go run ./app/gateway/cmd

# ─── Frontend ───────────────────────────────────────────────

.PHONY: fe-install
# Install frontend dependencies
fe-install:
	cd frontend && pnpm install

.PHONY: fe-dev
# Start frontend dev server
fe-dev:
	cd frontend && pnpm dev

.PHONY: fe-build
# Build frontend for production
fe-build:
	cd frontend && pnpm build

.PHONY: fe-test
# Run frontend unit tests
fe-test:
	cd frontend && pnpm test

.PHONY: fe-lint
# Lint frontend code
fe-lint:
	cd frontend && pnpm lint

# ─── Full build ─────────────────────────────────────────────

.PHONY: all
all: api conf wire build fe-build

# Container runtime detection (prefer podman)
CONTAINER_RUNTIME := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
COMPOSE_RUNTIME := $(shell command -v podman-compose 2>/dev/null || command -v docker-compose 2>/dev/null)

.PHONY: image-user
# Build user service container image
image-user:
	$(CONTAINER_RUNTIME) build --build-arg SERVICE=user --build-arg VERSION=$(VERSION) -t light-cloud-disk/user-service:$(VERSION) .

.PHONY: image-file
# Build file service container image
image-file:
	$(CONTAINER_RUNTIME) build --build-arg SERVICE=file --build-arg VERSION=$(VERSION) -t light-cloud-disk/file-service:$(VERSION) .

.PHONY: image-gateway
# Build gateway container image
image-gateway:
	$(CONTAINER_RUNTIME) build --build-arg SERVICE=gateway --build-arg VERSION=$(VERSION) -t light-cloud-disk/gateway:$(VERSION) .

.PHONY: images
# Build all container images
images: image-user image-file image-gateway

# Compose commands
.PHONY: up
up:
	$(COMPOSE_RUNTIME) up -d

.PHONY: down
down:
	$(COMPOSE_RUNTIME) down

.PHONY: logs
logs:
	$(COMPOSE_RUNTIME) logs -f

.PHONY: ps
ps:
	$(COMPOSE_RUNTIME) ps

# Infrastructure only (for local development)
.PHONY: infra-up
# Start infrastructure services (MySQL, Redis, Kafka, Consul)
infra-up:
	$(COMPOSE_RUNTIME) up -d mysql redis kafka consul

.PHONY: infra-down
infra-down:
	$(COMPOSE_RUNTIME) down mysql redis kafka consul

# Clean containers and volumes
.PHONY: clean-containers
clean-containers:
	$(COMPOSE_RUNTIME) down -v --remove-orphans

help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
