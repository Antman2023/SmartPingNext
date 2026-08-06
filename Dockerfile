# Frontend build stage
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Backend build stage
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy frontend to embed directory
COPY --from=frontend-builder /app/web/dist/ ./src/static/html/

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev

# Build backend with static linking
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-s -w -X main.Version=${VERSION}" -o smartping src/smartping.go

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /app/smartping ./

# Create directories for persistent data
RUN mkdir -p /app/conf /app/db /app/var /app/logs

# Set permissions
RUN chmod +x ./smartping

# Expose port
EXPOSE 8899

# Set environment variables
ENV TZ=Asia/Shanghai

# Create volume for persistent data
VOLUME ["/app/conf", "/app/db", "/app/logs"]

# Start the application
CMD ["./smartping"]
