# Stage 1: Build the static binary
FROM golang:alpine AS builder

WORKDIR /src

# Install CA certificates for secure HTTPS API calls
RUN apk add --no-cache ca-certificates git

# Cache module dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile fully static binary (CGO_ENABLED=0) with stripped symbols
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /bin/pr-review-go ./cmd/pr-review-go

# Stage 2: Minimal SCRATCH-based image
FROM scratch

# Root CA certificates for SSL/TLS connections to GitHub, Bitbucket, and LLM endpoints
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compiled static binary
COPY --from=builder /bin/pr-review-go /pr-review-go

# Expose HTTP port for the webhook service
EXPOSE 8080

ENTRYPOINT ["/pr-review-go"]
CMD ["serve", "--port=8080", "--host=0.0.0.0"]
