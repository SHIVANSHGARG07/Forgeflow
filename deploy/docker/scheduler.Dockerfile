
FROM golang:1.26-alpine AS builder

# Working directory inside the container
WORKDIR /app

# Copy only go.mod and go.sum first (for dependency caching).
# As long as these files don't change, Docker won't re-run the
# "go mod download" layer -> faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source code
COPY . .

# Build the binary, only the scheduler entrypoint.
# CGO_ENABLED=0 -> statically linked binary (no external C library needed at runtime)
# GOOS=linux -> always build for Linux (even if you build on Mac/Windows)
RUN CGO_ENABLED=0 GOOS=linux go build -o scheduler ./cmd/scheduler

# ---------- Stage 2: Final (small runtime image) ----------
# Only for running - the Go toolchain isn't needed here, the binary runs standalone.
FROM alpine:3.19

WORKDIR /app

# Copy only the compiled binary from the builder stage, nothing else
COPY --from=builder /app/scheduler .

# The gRPC server listens on this port (for documentation; actual binding happens via `docker run -p`)
EXPOSE 50051

# This command runs as soon as the container starts
CMD ["./scheduler"]
