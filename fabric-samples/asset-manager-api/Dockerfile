# ----- 1. Build Stage -----
# Use the official Golang image as the builder
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod ./
COPY go.sum ./

# Download the dependencies
RUN go mod download

# Copy the source code
COPY *.go ./

# Build the application, creating a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /asset-manager-api .

# ----- 2. Final Stage -----
# Use a minimal alpine image for the final container
FROM alpine:latest

# Add ca-certificates so our app can make TLS connections (like to the peer)
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /root/

# Copy the compiled binary from the 'builder' stage
COPY --from=builder /asset-manager-api .

# Expose port 8080 (the port our web server listens on)
EXPOSE 8080

# The command to run when the container starts
CMD ["./asset-manager-api"]