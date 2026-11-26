.PHONY: run build tidy
tidy:
	go mod tidy
run:
	go run ./cmd/server
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/hadir ./cmd/server
