FROM golang:1.26-alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -tags gotdx -o /warden-server ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /warden-server .
COPY config/config.yaml ./config/
EXPOSE 8080
CMD ["./warden-server"]
