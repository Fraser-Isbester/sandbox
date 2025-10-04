# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the k8s example
RUN CGO_ENABLED=0 GOOS=linux go build -o /consensus ./examples/k8s

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /consensus .

ENTRYPOINT ["/app/consensus"]
