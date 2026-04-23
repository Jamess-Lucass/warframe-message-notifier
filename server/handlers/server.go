package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
}

type Server struct {
	logger        *slog.Logger
	discordClient *discordgo.Session
	oauthConfig   *OAuthConfig
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

func NewServer(logger *slog.Logger, discordClient *discordgo.Session, oauthConfig *OAuthConfig, clientBaseURL string) *Server {
	return &Server{
		logger:        logger,
		discordClient: discordClient,
		oauthConfig:   oauthConfig,
		clientBaseURL: clientBaseURL,
		authCodes:     make(map[string]*TokenResponse),
	}
}

// generateRandomString returns a cryptographically random hex string.
func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
