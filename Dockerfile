# Build stage - use vendor mode to avoid network access during build
FROM docker.io/library/golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .

ARG SERVICE=user
ARG VERSION=dev

# SERVICE can be: user, file, gateway, worker
# worker builds from ./app/file/cmd/worker; others from ./app/${SERVICE}/cmd
RUN set -e; \
    if [ "$SERVICE" = "worker" ]; then \
      BUILD_PATH=./app/file/cmd/worker; \
    else \
      BUILD_PATH=./app/${SERVICE}/cmd; \
    fi; \
    CGO_ENABLED=0 GOOS=linux go build \
      -mod=vendor \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -o /app/server \
      ${BUILD_PATH}

# Prepare configs (user/file need YAML configs; gateway/worker read env only)
RUN mkdir -p /app/configs && \
    if [ "$SERVICE" != "worker" ]; then \
      cp -r app/${SERVICE}/configs/* /app/configs/ 2>/dev/null || true; \
    fi

# Runtime stage
FROM docker.io/library/alpine:3.21

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/configs/ ./configs/

ARG SERVICE=user
ENV SERVICE=${SERVICE}

ENV TZ=Asia/Shanghai

EXPOSE 8080 9001 9002

# gateway/worker: run directly; kratos services (user/file): use -conf flag
CMD ["/bin/sh", "-c", \
    "if [ \"$SERVICE\" = \"gateway\" ] || [ \"$SERVICE\" = \"worker\" ]; then \
        /app/server; \
    else \
        /app/server -conf /app/configs/; \
    fi"]
