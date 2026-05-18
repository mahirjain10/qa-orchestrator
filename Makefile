.PHONY: build run test clean tidy lint

build:
	go build -o ./bin/qa-orchestrator ./apps/tui/cmd/main.go

run: build
	./bin/qa-orchestrator

test:
	go test ./...

clean:
	rm -rf ./bin/qa-orchestrator
	go clean

tidy:
	go mod tidy

lint:
	go vet ./...
	go fmt ./...