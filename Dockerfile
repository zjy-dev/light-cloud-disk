# Build stage - use vendor mode to avoid network access during build
FROM docker.io/library/golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .

ARG SERVICE=user
ARG VERSION=dev

# SERVICE can be: user, file, gateway
RUN CGO_ENABLED=0 GOOS=linux go build \
    -mod=vendor \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/${SERVICE}-service \
    ./app/${SERVICE}/cmd

# Runtime stage - alpine base without apk installs (CA certs copied from builder)
FROM docker.io/library/alpine:3.21

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

COPY --from=builder /app/*-service .

ARG SERVICE=user
ENV SERVICE=${SERVICE}

# Copy configs only for kratos services (user, file)
# Gateway reads config from env vars only
COPY app/${SERVICE}/configs/ ./configs/

ENV TZ=Asia/Shanghai

EXPOSE 8080 9001 9002

# Gateway uses -addr flag, Kratos services use -conf flag
CMD ["/bin/sh", "-c", \
    "if [ \"$SERVICE\" = \"gateway\" ]; then \
        /app/gateway-service; \
    else \
        /app/${SERVICE}-service -conf /app/configs/; \
    fi"]
