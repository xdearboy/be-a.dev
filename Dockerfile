# Build stage
FROM golang:alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/registry ./cmd/registry

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/registry /app/registry
COPY config/ /app/config/
COPY domains/ /app/domains/

EXPOSE 8080
ENTRYPOINT ["/app/registry"]
CMD ["serve", "--addr", ":8080", "--domains-dir", "/app/domains"]
