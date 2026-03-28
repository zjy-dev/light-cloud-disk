# Build stage — golang:1.25 (debian) ships gcc + libc-dev out of the box,
# eliminating the need for network access (apk add) during build.
# This ensures compatibility with podman's default bridge network which may
# lack external connectivity in some environments.
FROM docker.io/library/golang:1.25 AS builder

# Clear proxy env vars inherited from host build environment
ENV HTTP_PROXY="" HTTPS_PROXY="" http_proxy="" https_proxy="" ALL_PROXY="" all_proxy=""

WORKDIR /src

COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .

ARG SERVICE=user
ARG VERSION=dev

# SERVICE can be user, file, gateway, or worker
# worker builds from ./app/file/cmd/worker, others build from ./app/${SERVICE}/cmd
RUN set -e; \
    if [ "$SERVICE" = "worker" ]; then \
      BUILD_PATH=./app/file/cmd/worker; \
    else \
      BUILD_PATH=./app/${SERVICE}/cmd; \
    fi; \
    CGO_ENABLED=1 GOOS=linux go build \
      -mod=vendor \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -o /app/server \
      ${BUILD_PATH}

# Prepare config files (user/file use YAML, gateway/worker mainly read env vars)
RUN mkdir -p /app/configs && \
    if [ "$SERVICE" != "worker" ]; then \
      cp -r app/${SERVICE}/configs/* /app/configs/ 2>/dev/null || true; \
    fi

# Runtime stage — use debian-slim to match glibc from build stage (CGO/SQLite)
FROM docker.io/library/debian:bookworm-slim

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/configs/ ./configs/

# Ensure directories exist for local mode (SQLite DB + file store)
RUN mkdir -p /app/data /app/tmp /app/store

ARG SERVICE=user
ENV SERVICE=${SERVICE}

ENV TZ=Asia/Shanghai

EXPOSE 8080 9001 9002

# gateway/worker run directly, Kratos services (user/file) use -conf for config path
CMD ["/bin/sh", "-c", \
    "if [ \"$SERVICE\" = \"gateway\" ] || [ \"$SERVICE\" = \"worker\" ]; then \
        /app/server; \
    else \
        /app/server -conf /app/configs/; \
    fi"]
