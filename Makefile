GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	Git_Bash=$(subst \,/,$(subst cmd\,bindiff\git-bash.exe,$(dir $(shell where git))))
	INTERNAL_PROTO_FILES=$(shell $(Git_Bash) -c "find internal -name *.proto")
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
else
	INTERNAL_PROTO_FILES=$(shell find internal -name "*.proto")
	API_PROTO_FILES=$(shell find api -name "*.proto")
endif

.PHONY: init
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: api
api:
	protoc --proto_path=./api \
		   --proto_path=./third_party \
		   --go_out=paths=source_relative:./api \
		   --go-http_out=paths=source_relative:./api \
		   --go-grpc_out=paths=source_relative:./api \
		   $(API_PROTO_FILES)

.PHONY: conf
conf:
	protoc --proto_path=./internal \
		   --go_out=paths=source_relative:./internal \
		   $(INTERNAL_PROTO_FILES)

.PHONY: build
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...

.PHONY: generate
generate:
	go generate ./...

.PHONY: wire
wire:
	cd cmd/user && wire
	cd cmd/file && wire

.PHONY: test
test:
	go test -v -cover ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: run-user
run-user:
	go run ./cmd/user -conf ./configs/

.PHONY: run-file
run-file:
	go run ./cmd/file -conf ./configs/

.PHONY: all
all: api conf generate build

# Container runtime detection (prefer podman)
CONTAINER_RUNTIME := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
COMPOSE_RUNTIME := $(shell command -v podman-compose 2>/dev/null || command -v docker-compose 2>/dev/null)

# Container image build
.PHONY: image-user
image-user:
	$(CONTAINER_RUNTIME) build --build-arg SERVICE=user --build-arg VERSION=$(VERSION) -t light-cloud-disk/user-service:$(VERSION) .

.PHONY: image-file
image-file:
	$(CONTAINER_RUNTIME) build --build-arg SERVICE=file --build-arg VERSION=$(VERSION) -t light-cloud-disk/file-service:$(VERSION) .

.PHONY: images
images: image-user image-file

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
infra-up:
	$(COMPOSE_RUNTIME) up -d mysql redis kafka

.PHONY: infra-down
infra-down:
	$(COMPOSE_RUNTIME) down mysql redis kafka

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
