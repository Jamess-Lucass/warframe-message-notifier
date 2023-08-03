.PHONY: client
client:
	dotenv -- go run ./client/cmd

.PHONY: server
server:
	dotenv -- go run ./server/cmd

.PHONY: compose
compose:
	docker compose up -d --build

.PHONY: build-client
build-client:
	go build -ldflags "-X main.apiBaseUrl=$(API_BASE_URL)" -o bin/warframe-message-notifier ./client/cmd