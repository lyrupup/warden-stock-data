FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /warden-server ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /warden-server .
COPY config/config.yaml ./config/
EXPOSE 8080
CMD ["./warden-server"]
