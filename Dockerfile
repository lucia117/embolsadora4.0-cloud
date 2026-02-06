# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application in the current directory
RUN CGO_ENABLED=0 GOOS=linux go build -o embolsadora-api ./cmd/api

# Verify the binary was created
RUN ls -la embolsadora-api

# Final stage
FROM alpine:3.18

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/embolsadora-api .

# Make the binary executable
RUN chmod +x /app/embolsadora-api

# Verify the binary exists and is executable
RUN ls -la /app/embolsadora-api

# Expose the port the app runs on
EXPOSE 8080

# Command to run the application
CMD ["/app/embolsadora-api"]
