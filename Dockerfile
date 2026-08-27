# Multi-stage Dockerfile for Gruntdeck v1.0.0
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build tools
RUN apk add --no-cache git gcc musl-dev

# Download Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY . .

# Build binaries for server and executor
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/executor ./cmd/executor

# Runtime Image
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata openssh-client

# Job steps may only read source_path files from here (override: GRUNTDECK_SCRIPT_DIR).
# Mount your scripts into this directory.
RUN mkdir -p /app/scripts

# Copy binaries and web assets
COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/bin/executor /app/executor
COPY --from=builder /app/web /app/web
COPY --from=builder /app/internal/migrations /app/internal/migrations

EXPOSE 8080

CMD ["/app/server"]
