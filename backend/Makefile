.PHONY: test build run tidy

test:
	go test ./... -count=1

build:
	go build -o bin/warden-server ./cmd/server

run:
	go run ./cmd/server

tidy:
	go mod tidy
