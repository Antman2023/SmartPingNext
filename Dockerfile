# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git nodejs npm

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build frontend
WORKDIR /app/web
RUN npm install && npm run build

# Copy frontend to embed directory
WORKDIR /app
RUN rm -rf src/static/html && mkdir -p src/static/html && cp -r web/dist/* src/static/html/

# Build backend with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o smartping src/smartping.go

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
