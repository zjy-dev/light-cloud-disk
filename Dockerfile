# Build stage
FROM docker.io/library/golang:1.25-alpine AS builder

RUN apk add --no-cache make git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE=user
ARG VERSION=dev

# SERVICE can be: user, file, gateway
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/${SERVICE}-service \
    ./app/${SERVICE}/cmd

# Runtime stage
FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/*-service .

ARG SERVICE=user
COPY app/${SERVICE}/configs/ ./configs/

ENV TZ=Asia/Shanghai

EXPOSE 8080 9001 9002

ENV SERVICE=${SERVICE}

CMD ["/bin/sh", "-c", "/app/${SERVICE}-service -conf /app/configs/"]
