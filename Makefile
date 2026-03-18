.PHONY: build run clean docker-up docker-down

build:
	go build -o tui-agar ./cmd/tui-agar

run:
	go run ./cmd/tui-agar

clean:
	rm -f tui-agar

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

test:
	go test ./...

# Start server and run client
dev: docker-up
	sleep 3 && go run ./cmd/tui-agar
