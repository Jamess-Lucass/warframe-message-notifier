package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Jamess-Lucass/warframe-message-notifier/server/handlers"
	"github.com/bwmarrin/discordgo"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	client, err := discordgo.New(fmt.Sprintf("Bot %s", os.Getenv("DISCORD_BOT_TOKEN")))
	if err != nil {
		return fmt.Errorf("unable to create discord client: %w", err)
	}

	// OAuth2 config using the canonical Discord OAuth2 URLs:
	// Auth:  https://discord.com/oauth2/authorize
	// Token: https://discord.com/api/oauth2/token
	// See: https://docs.discord.com/developers/topics/oauth2#shared-resources-oauth2-urls
	oauthConfig := &handlers.OAuthConfig{
		ClientID:     os.Getenv("DISCORD_BOT_CLIENT_ID"),
		ClientSecret: os.Getenv("DISCORD_BOT_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("DISCORD_BOT_REDIRECT_URI"),
		AuthURL:      "https://discord.com/oauth2/authorize",
		TokenURL:     "https://discord.com/api/oauth2/token",
	}

	clientBaseURL := os.Getenv("CLIENT_API_BASE_URL")

	server := handlers.NewServer(logger, client, oauthConfig, clientBaseURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.Start(ctx)
}
