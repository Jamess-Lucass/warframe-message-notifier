# warframe-message-notifier

> [!IMPORTANT]  
> The server used to host the Discord bot is no longer running. You may however create your own Discord bot and self-host both the client and server.

This project detects direct messages from users in Warframe and sends you a Discord DM notification.

## Motivation

Warframe has a trading feature enhanced by [Warframe Market](https://warframe.market/), a third-party site where players list buy and sell offers. You can find a seller, message them in-game, and perform the trade. But if you're waiting for messages, you might want to minimize the game, play something else, or watch Netflix. Without constantly watching the game, you'd miss incoming messages — this project solves that.

## How it works

When you receive a new direct message in-game, Warframe creates a new tab in the chat window. This event is logged in the `EE.log` file. The client watches this log file, detects new chat tabs, extracts the sender's username, and sends a request to the server. The server then sends you a Discord DM via the bot.

You'll only be notified for new conversations. If a message arrives in an already open chat tab, it won't be logged and won't trigger a notification. Only the username is captured — message content is not logged by Warframe.

## Prerequisites

- A [Discord application](https://discord.com/developers/applications) with a bot
- The bot token, client ID, and client secret from your Discord application
- An OAuth2 redirect URI set to `http://localhost:8080/api/v1/discord/authorize/callback` in your Discord application
- At least one mutual server between your Discord account and the bot

## Setup

### 1. Configure environment variables

```bash
cp .env.example .env
```

Fill in the `.env` file:

| Variable | Description |
|---|---|
| `DISCORD_BOT_TOKEN` | Your Discord bot token |
| `DISCORD_BOT_CLIENT_ID` | Your Discord application client ID |
| `DISCORD_BOT_CLIENT_SECRET` | Your Discord application client secret |
| `DISCORD_BOT_REDIRECT_URI` | `http://localhost:8080/api/v1/discord/authorize/callback` |
| `CLIENT_API_BASE_URL` | `http://localhost:8081` |
| `API_BASE_URL` | `http://localhost:8080` |
| `WF_EE_LOG_FILE_PATH` | Path to your Warframe `EE.log` file |

The `EE.log` file is typically located at:
- **Windows:** `%LOCALAPPDATA%/Warframe/EE.log`

### 2. Run the project

#### Docker Compose

```bash
docker compose up -d --build
```

Or using the Makefile:

```bash
make compose
```

#### Pre-built images from GHCR

```bash
# Terminal 1 - server
docker run --rm -p 8080:8080 --env-file .env \
  -e CLIENT_API_BASE_URL=http://localhost:8081 \
  -e DISCORD_BOT_REDIRECT_URI=http://localhost:8080/api/v1/discord/authorize/callback \
  ghcr.io/jamess-lucass/warframe-message-notifier-server:main
```

**Powershell:**

```bash
# Terminal 2 - client
docker run --rm -p 8081:8081 \
  -e API_BASE_URL=http://host.docker.internal:8080 \
  -v $env:LocalAppData/Warframe/EE.log:/tmp/warframe/EE.log \
  ghcr.io/jamess-lucass/warframe-message-notifier-client:main
```

**Command Prompt:**

```bash
# Terminal 2 - client
docker run --rm -p 8081:8081 \
  -e API_BASE_URL=http://host.docker.internal:8080 \
  -v %LOCALAPPDATA%/Warframe/EE.log:/tmp/warframe/EE.log \
  ghcr.io/jamess-lucass/warframe-message-notifier-client:main
```

#### Without Docker

Requires [Go 1.26+](https://go.dev/dl/) and the [`dotenv`](https://github.com/joho/godotenv) CLI.

```bash
# Terminal 1 - server
make server

# Terminal 2 - client
make client
```

### 3. Authenticate with Discord

Open the URL printed in the client logs:

```
Please authenticate with Discord via: http://localhost:8080/api/v1/discord/authorize
```

Authorize with Discord and you're done. The client will now watch your log file and send you a Discord DM when you receive an in-game message.

## Updating

### Docker

```bash
docker compose pull
docker compose up -d
```

### Executable

Download the new release version from [releases](https://github.com/Jamess-Lucass/warframe-message-notifier/releases) and run it with the required environment variables set.
