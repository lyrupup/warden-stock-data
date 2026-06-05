.PHONY: test build build-gotdx run backfill tidy

test:
	go test ./... -count=1

build:
	go build -o bin/warden-server ./cmd/server

build-gotdx:
	go build -tags gotdx -o bin/warden-server ./cmd/server

run:
	go run ./cmd/server

backfill:
	go run ./cmd/backfill -years=5

tidy:
	go mod tidy
