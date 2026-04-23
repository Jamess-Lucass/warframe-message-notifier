package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/oauth2"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

type SendChannelMessageRequest struct {
	Content string `json:"content" validate:"required,max=2000"`
}

type ExchangeAuthCodeRequest struct {
	Code string `json:"code" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Default().Error("failed to write JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Message: msg})
}

// Authorize initiates the Discord OAuth2 Authorization Code Grant flow.
// Redirects the user to Discord's authorization page.
//
// Discord OAuth2 docs: https://docs.discord.com/developers/topics/oauth2#authorization-code-grant
func (s *Server) Authorize(w http.ResponseWriter, r *http.Request) {
	redirectURL := s.oauthConfig.AuthCodeURL("", oauth2.SetAuthURLParam("prompt", "consent"))
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// AuthorizeCallback handles the redirect from Discord after user authorization.
// It exchanges the authorization code for tokens via the oauth2 package, stores
// the token behind a short-lived auth code, and redirects to the client.
//
// Discord OAuth2 docs: https://docs.discord.com/developers/topics/oauth2#authorization-code-grant
func (s *Server) AuthorizeCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Missing authorization code")
		return
	}

	// Exchange the Discord authorization code for access + refresh tokens.
	// The oauth2 package handles the POST to https://discord.com/api/oauth2/token
	// with grant_type=authorization_code, code, redirect_uri, client_id, client_secret.
	oauthToken, err := s.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("failed to exchange authorization code for token", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to exchange authorization code")
		return
	}

	token := &TokenResponse{
		AccessToken:  oauthToken.AccessToken,
		TokenType:    oauthToken.TokenType,
		ExpiresIn:    int64(time.Until(oauthToken.Expiry).Seconds()),
		RefreshToken: oauthToken.RefreshToken,
	}

	// Generate a short-lived auth code so we don't pass the Discord access token
	// through a browser redirect URL (which leaks in browser history, logs, etc.)
	authCode, err := generateRandomString(32)
	if err != nil {
		s.logger.Error("failed to generate auth code", "error", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	s.authCodesMu.Lock()
	s.authCodes[authCode] = token
	s.authCodesMu.Unlock()

	// Clean up the auth code after 60 seconds if unused
	go func() {
		time.Sleep(60 * time.Second)
		s.authCodesMu.Lock()
		delete(s.authCodes, authCode)
		s.authCodesMu.Unlock()
	}()

	redirectURI := fmt.Sprintf("%s/api/v1/oauth/callback?code=%s", s.clientBaseURL, url.QueryEscape(authCode))
	http.Redirect(w, r, redirectURI, http.StatusTemporaryRedirect)
}

// ExchangeAuthCode allows the client to exchange a short-lived auth code
// (received via redirect) for the actual Discord access and refresh tokens.
func (s *Server) ExchangeAuthCode(w http.ResponseWriter, r *http.Request) {
	request := ExchangeAuthCodeRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.validator.Struct(request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.authCodesMu.Lock()
	token, exists := s.authCodes[request.Code]
	if exists {
		delete(s.authCodes, request.Code)
	}
	s.authCodesMu.Unlock()

	if !exists {
		writeError(w, http.StatusBadRequest, "Invalid or expired auth code")
		return
	}

	writeJSON(w, http.StatusOK, token)
}

// RefreshToken exchanges a Discord refresh token for a new access token.
// Uses the oauth2 package's TokenSource to handle the refresh via
// POST https://discord.com/api/oauth2/token with grant_type=refresh_token.
//
// Discord OAuth2 docs: https://docs.discord.com/developers/topics/oauth2#authorization-code-grant-refresh-token-exchange-example
func (s *Server) RefreshToken(w http.ResponseWriter, r *http.Request) {
	request := RefreshTokenRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.validator.Struct(request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Create an expired token with the refresh token set so that
	// TokenSource automatically triggers a refresh.
	expiredToken := &oauth2.Token{
		RefreshToken: request.RefreshToken,
	}

	newToken, err := s.oauthConfig.TokenSource(r.Context(), expiredToken).Token()
	if err != nil {
		s.logger.Error("failed to refresh token", "error", err)
		writeError(w, http.StatusUnauthorized, "Failed to refresh token")
		return
	}

	writeJSON(w, http.StatusOK, TokenResponse{
		AccessToken:  newToken.AccessToken,
		TokenType:    newToken.TokenType,
		ExpiresIn:    int64(time.Until(newToken.Expiry).Seconds()),
		RefreshToken: newToken.RefreshToken,
	})
}

// SendChannelMessage sends a DM to the authenticated Discord user via the bot.
// The client provides a Bearer token (Discord access token) in the Authorization header.
// The server uses it to identify the user, then uses the bot session to send the DM.
func (s *Server) SendChannelMessage(w http.ResponseWriter, r *http.Request) {
	request := SendChannelMessageRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.validator.Struct(request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	client, err := discordgo.New(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid authorization")
		return
	}

	user, err := client.User("@me")
	if err != nil {
		s.logger.Error("failed to fetch user details", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ch, err := s.discordClient.UserChannelCreate(user.ID)
	if err != nil {
		s.logger.Error("failed to create DM channel", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := s.discordClient.ChannelMessageSend(ch.ID, request.Content); err != nil {
		s.logger.Error("failed to send DM", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.logger.Info("Sent Discord DM", "user_id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}
