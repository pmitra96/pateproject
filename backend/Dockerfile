# Build stage
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev make

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application via Makefile
RUN make build

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates make

# Copy the binary and Makefile from builder
COPY --from=builder /app/main .
COPY --from=builder /app/Makefile .

EXPOSE 8080

CMD ["make", "prod-run"]
