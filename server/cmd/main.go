package main

import (
	"fmt"
	"os"

	"github.com/Jamess-Lucass/warframe-message-notifier/server/handlers"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := logger.Sync(); err != nil {
			logger.Sugar().Warnf("could not flush: %v", err)
		}
	}()

	client, err := discordgo.New(fmt.Sprintf("Bot %s", os.Getenv("DISCORD_BOT_TOKEN")))
	if err != nil {
		logger.Sugar().Fatalf("unable to create discord client: %v", err)
	}

	server := handlers.NewServer(logger, client)

	if err := server.Start(); err != nil {
		logger.Sugar().Fatalf("error starting web server: %v", err)
	}
}
