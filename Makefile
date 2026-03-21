.PHONY: build run clean test fmt vet lint install-hooks

build:
	go build -o tui-agar ./cmd/tui-agar

run:
	go run ./cmd/tui-agar

clean:
	rm -f tui-agar debug-pty

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...

install-hooks:
	git config core.hooksPath .githooks
