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
	"github.com/go-playground/validator/v10"
	"golang.org/x/oauth2"
)

type config struct {
	BotToken      string `validate:"required"`
	ClientID      string `validate:"required"`
	ClientSecret  string `validate:"required"`
	RedirectURI   string `validate:"required"`
	ClientBaseURL string `validate:"required"`
}

func loadConfig() (*config, error) {
	cfg := &config{
		BotToken:      os.Getenv("DISCORD_BOT_TOKEN"),
		ClientID:      os.Getenv("DISCORD_BOT_CLIENT_ID"),
		ClientSecret:  os.Getenv("DISCORD_BOT_CLIENT_SECRET"),
		RedirectURI:   os.Getenv("DISCORD_BOT_REDIRECT_URI"),
		ClientBaseURL: os.Getenv("CLIENT_API_BASE_URL"),
	}

	if err := validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("missing required environment variables: %w", err)
	}

	return cfg, nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	client, err := discordgo.New(fmt.Sprintf("Bot %s", cfg.BotToken))
	if err != nil {
		return fmt.Errorf("unable to create discord client: %w", err)
	}

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       []string{"identify"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://discord.com/oauth2/authorize",
			TokenURL:  "https://discord.com/api/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}

	server := handlers.NewServer(logger, client, oauthConfig, cfg.ClientBaseURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.Start(ctx)
}
