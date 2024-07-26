# warframe-message-notifier

> [!IMPORTANT]  
> The server used to host the discord bot is no longer running. You will not receive any discord messages. You may however create your own discord bot and reuse the server code.

This project will detect direct messages from users in Warframe, then send a Discord DM notifying you.

## Motiviation

Warframe has a trading feature, which is only made better by a third-party website named Warframe Market, which allows people to list WTS and WTB offers on their site, you can look who is selling what, at what price, then message them in game and perform the trade. However, if you're just in the mood to trade, you may sit there, waiting for messages in game, instead, you may want to go and do something else, minimize the app, play another game, watch netflix etc, many reasons for you to miss an in-game message unless you're constantly looking at the game, well, no more.

## How it works

After running the client, you'll be asked to authenticate with Discord, this is to know what Discord user to send a DM to. After successfully authenticating, the application reads the Warframe EE.log file and when it detects a log entry for a in-game message, it will parse that line, extract the username, then send a request to the server, which will send you a Discord DM from the Bot.

When you receive a new message in game, it creates a new tab within the chat window, this event is logged inside the EE.log file, we can utilize this to detect new in-game messages, this means, you'll only receieve a Discord DM if it's a new DM, if a message is received inside an already existing tab in the chat window, it will not be logged and thus, won't be picked up.

It only logs that a new tab was added to the chat window with the title of the tab, which happens to be the username, and this is how we can extract the username, the message content is not logged to file, so we can only notify that someone sent you an in-game message.

## Prerequisites

- You must have at least one mutual server with the Discord bot.

> You may invite the Discord bot to a server you have permission to with this link [https://discord.com/oauth2/authorize?client_id=1132776281474355271&permissions=0&scope=bot](https://discord.com/oauth2/authorize?client_id=1132776281474355271&permissions=0&scope=bot)

## Setup

### From Docker

> The below assumes you have Docker installed and have a basic understanding of it.

1. Run the Docker container.

   > Run the below commands whether you are running the container from the default Windows Command Prompt or from Powershell, they both have different environment variables to access your local app data, that is the only difference between the commands. It has to run on port 8081 and the volume path is for the EE.log file, for the majority of users, this will be what is in the commands below.

- Powershell

  ```bash
  docker run --rm -p 8081:8081 --name warframe-message-notifier -v $env:LocalAppData/Warframe/EE.log:/tmp/warframe/EE.log ghcr.io/jamess-lucass/warframe-message-notifier-client:main
  ```

- Command Prompt

  ```bash
  docker run --rm -p 8081:8081 --name warframe-message-notifier -v %LOCALAPPDATA%/Warframe/EE.log:/tmp/warframe/EE.log ghcr.io/jamess-lucass/warframe-message-notifier-client:main
  ```

### From executable

1. Download the executable from your desired release version [here](https://github.com/Jamess-Lucass/warframe-message-notifier/releases)

2. Run the executable.

   > You will need to set the environment variable `WF_EE_LOG_FILE_PATH` so the app knows what log file to read.

- Powershell

  ```bash
  $env:WF_EE_LOG_FILE_PATH="$env:LocalAppData/Warframe/EE.log"; .\warframe-message-notifier.exe
  ```

- Command Prompt

  ```bash
  set "WF_EE_LOG_FILE_PATH=%LOCALAPPDATA%/Warframe/EE.log" && warframe-message-notifier.exe
  ```

## How to use

> Please complete the 'Setup' section first. This assumes you have the app running.

1. Authenticate with discord by navigating to the URL presented in the command prompt, this will be presented after the text "Please authenticate with discord via: \<URL>".

   > You need to authenticate with discord so the application knows who to send the Discord DM to.

2. Wait to receive a direct message in game and you'll be sent a direct message on Discord notifying you :)

## Updating

### Docker

1. Pull the latest image

   ```bash
   docker pull ghcr.io/jamess-lucass/warframe-message-notifier-client:main
   ```

### Executable

1. Download the new release version for your operating system.

2. Run the new version following the [From executable](#from-executable) section.
