FROM golang:alpine AS builder

WORKDIR /app

# Install git (required for fetching dependencies)
RUN apk add --no-cache git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# CGO_ENABLED=0 is preferred for modernc.org/sqlite (pure Go) and static binaries
# -trimpath: removes file system paths from the executable
# -ldflags="-s -w": strips debug information for smaller binary size
RUN CGO_ENABLED=0 go build \
	-trimpath \
	-ldflags="-s -w" \
	-o workflow .

# -----------------------------------------------------------------------------
# Run Stage
# -----------------------------------------------------------------------------
FROM alpine:latest

WORKDIR /app

# Install CA certificates (required for Discord HTTPS) and Timezone data
RUN apk add --no-cache ca-certificates tzdata

# Copy the compiled binary from builder
COPY --from=builder /app/workflow .

# Copy static frontend files (Dashboard)
COPY --from=builder /app/pb_public ./pb_public

# Create data directory
RUN mkdir pb_data

# Expose the PocketBase port
# Dokploy/Traefik will use this to route traffic
EXPOSE 8090

# Define volume for persistence
# Ensure you map this volume in Dokploy settings to persist DB
VOLUME /app/pb_data

# Start the application binding to all interfaces
CMD ["./workflow", "serve", "--http=0.0.0.0:8090"]
