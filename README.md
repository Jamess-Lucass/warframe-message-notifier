# warframe-message-notifier

This project will detect direct messages from users in Warframe, then send a Discord DM notifying you.

## Motiviation

Warframe has a trading feature, which is only made better by a third-party website named Warframe Market, which allows people to list WTS and WTB offers on their site, you can look who is selling what, at what price, then message them in game and perform the trade. However, if you're just in the mood to trade, you may sit there, waiting for messages in game, instead, you may want to go and do something else, minimize the app, play another game, watch netflix etc, many reasons for you to miss an in-game message unless you're constantly looking at the game, well, no more.

## How it works

After running the client, you'll be asked to authenticate with Discord, this is to know what Discord user to send a DM to. After successfully authenticating, the application tails the Warframe EE.log file and when it detects a log entry for a in-game message, it will parse that line, extract the username, then send a request to the server, which will send you a Discord DM from the Bot.

## Prerequisites

You must either join a Discord server with the Bot in, or invite the Bot to your discord server.

_This is a security feature by Discord, you must be in a server with the Bot, in order for it to have permission to DM you._
