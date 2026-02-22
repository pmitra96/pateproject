# Build stage
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o main ./cmd/server/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (like certificates)
RUN apk add --no-cache ca-certificates

# Copy the binary from builder
COPY --from=builder /app/main .

# Copy resources if needed (e.g., templates, static files)
# COPY --from=builder /app/resources ./resources

EXPOSE 8080

CMD ["./main"]
