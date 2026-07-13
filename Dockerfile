# syntax=docker/dockerfile:1

# Stage 1: Build the React frontend
# digest pinned 2026-07-13 — node:22-bookworm-slim
FROM node:22-bookworm-slim@sha256:a149cd71dccd68704a07d4e4ca3e610c27301852b0f556865cfdb6e2856f8bed AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build the Go binary with embedded frontend
# digest pinned 2026-07-13 — golang:1.25-bookworm
FROM golang:1.25-bookworm@sha256:544ae22bc85dae968aff6777784957963228cf4834adc6bf674ff72cf9e83335 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/glyphdeck ./cmd/glyphdeck

# Stage 3: Minimal runtime image
# digest pinned 2026-07-13 — debian:bookworm-slim
FROM debian:bookworm-slim@sha256:1def178129dfb5f24db43afbf2fcac04530012e3264ba4ff81c71184e17a9ee4

# Install runtime dependencies: CA certificates, SSH client (for remote SSH targets), curl (for healthcheck).
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        openssh-client \
        curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --shell /usr/sbin/nologin glyphdeck \
    && mkdir -p /home/glyphdeck/data \
    && chown glyphdeck:glyphdeck /home/glyphdeck/data

# Copy the compiled binary.
COPY --from=builder /out/glyphdeck /usr/local/bin/glyphdeck

# Copy legal documents.
COPY LICENSE /usr/local/share/glyphdeck/LICENSE
COPY COMMERCIAL-LICENSING.md /usr/local/share/glyphdeck/COMMERCIAL-LICENSING.md

USER glyphdeck
WORKDIR /home/glyphdeck

# Set the default data directory inside the container home.
# Override with a volume mount for persistence.
ENV GLYPHDECK_DATA_DIR=/home/glyphdeck/data

EXPOSE 8756

# exec-form ENTRYPOINT for proper signal handling.
# SIGTERM is handled by the Go HTTP server's graceful shutdown.
ENTRYPOINT ["glyphdeck"]
