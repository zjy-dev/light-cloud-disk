# Build stage uses vendor mode to avoid external network access
FROM docker.io/library/golang:1.25-alpine AS builder

# gcc + musl-dev needed for CGO (SQLite driver)
RUN apk add --no-cache gcc musl-dev

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

# Runtime stage
FROM docker.io/library/alpine:3.21

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
