.PHONY: client
client:
	dotenv -- go run ./client/cmd

.PHONY: server
server:
	dotenv -- go run ./server/cmd

.PHONY: compose
compose:
	docker compose up -d --build