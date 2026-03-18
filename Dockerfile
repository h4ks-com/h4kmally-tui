# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o tui-agar ./cmd/tui-agar

# Runtime stage  
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/tui-agar .

ENTRYPOINT ["./tui-agar"]
CMD ["-name", "Player"]
