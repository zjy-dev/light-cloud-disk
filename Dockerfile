# Build stage
FROM docker.io/library/golang:1.22-alpine AS builder

RUN apk add --no-cache make git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE=user
ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/${SERVICE}-service \
    ./cmd/${SERVICE}

# Runtime stage
FROM docker.io/library/alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/*-service .
COPY configs/ ./configs/

ENV TZ=Asia/Shanghai

EXPOSE 8000 9000

ARG SERVICE=user
ENV SERVICE=${SERVICE}

CMD ["/bin/sh", "-c", "/app/${SERVICE}-service -conf /app/configs/"]
