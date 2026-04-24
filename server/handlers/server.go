package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/go-playground/validator/v10"
	"golang.org/x/oauth2"
)

type Server struct {
	logger        *slog.Logger
	validator     *validator.Validate
	discordClient *discordgo.Session
	oauthConfig   *oauth2.Config
	clientBaseURL string

	// authCodes stores short-lived authorization codes that the client
	// exchanges for the actual Discord token. This avoids passing the
	// Discord access token through a browser redirect URL.
	// Maps code -> token response.
	authCodes   map[string]*TokenResponse
	authCodesMu sync.Mutex
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func NewServer(logger *slog.Logger, discordClient *discordgo.Session, oauthConfig *oauth2.Config, clientBaseURL string) *Server {
	return &Server{
		logger:        logger,
		validator:     validator.New(),
		discordClient: discordClient,
		oauthConfig:   oauthConfig,
		clientBaseURL: clientBaseURL,
		authCodes:     make(map[string]*TokenResponse),
	}
}

func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
