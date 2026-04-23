package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/bwmarrin/discordgo"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

type SendChannelMessageRequest struct {
	Content string `json:"content"`
}

type ExchangeAuthCodeRequest struct {
	Code string `json:"code"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Best effort -- if we can't write the response, log and move on.
		// The status code has already been sent at this point.
		slog.Default().Error("failed to write JSON response", "error", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Message: msg})
}

// Authorize initiates the Discord OAuth2 Authorization Code Grant flow.
// Redirects the user to Discord's authorization page.
//
// Discord OAuth2 docs: https://docs.discord.com/developers/topics/oauth2#authorization-code-grant
func (s *Server) Authorize(w http.ResponseWriter, r *http.Request) {
	params := url.Values{
		"client_id":     {s.oauthConfig.ClientID},
		"scope":         {"identify"},
		"redirect_uri":  {s.oauthConfig.RedirectURL},
		"response_type": {"code"},
		"prompt":        {"consent"},
	}

	redirectURL := fmt.Sprintf("%s?%s", s.oauthConfig.AuthURL, params.Encode())
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// AuthorizeCallback handles the redirect from Discord after user authorization.
// It exchanges the authorization code for tokens via POST to
// https://discord.com/api/oauth2/token, stores the token behind a short-lived
// auth code, and redirects to the client with that code.
//
// Discord OAuth2 docs: https://docs.discord.com/developers/topics/oauth2#authorization-code-grant
func (s *Server) AuthorizeCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Missing authorization code")
		return
	}

	token, err := s.exchangeCodeForToken(code)
	if err != nil {
		s.logger.Error("failed to exchange authorization code for token", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to exchange authorization code")
		return
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

	if request.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
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
// Calls POST https://discord.com/api/oauth2/token with grant_type=refresh_token.
//
// Discord OAuth2 docs: https://docs.discord.com/developers/topics/oauth2#authorization-code-grant-refresh-token-exchange-example
func (s *Server) RefreshToken(w http.ResponseWriter, r *http.Request) {
	request := RefreshTokenRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if request.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	token, err := s.refreshDiscordToken(request.RefreshToken)
	if err != nil {
		s.logger.Error("failed to refresh token", "error", err)
		writeError(w, http.StatusUnauthorized, "Failed to refresh token")
		return
	}

	writeJSON(w, http.StatusOK, token)
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

	if request.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	if len(request.Content) > 2000 {
		writeError(w, http.StatusBadRequest, "content must be 2000 characters or less")
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

// exchangeCodeForToken exchanges a Discord authorization code for access + refresh tokens.
// POST https://discord.com/api/oauth2/token with Content-Type: application/x-www-form-urlencoded
func (s *Server) exchangeCodeForToken(code string) (*TokenResponse, error) {
	return s.postTokenRequest(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {s.oauthConfig.RedirectURL},
		"client_id":     {s.oauthConfig.ClientID},
		"client_secret": {s.oauthConfig.ClientSecret},
	})
}

// refreshDiscordToken exchanges a refresh token for a new access token.
// POST https://discord.com/api/oauth2/token with Content-Type: application/x-www-form-urlencoded
func (s *Server) refreshDiscordToken(refreshToken string) (*TokenResponse, error) {
	return s.postTokenRequest(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {s.oauthConfig.ClientID},
		"client_secret": {s.oauthConfig.ClientSecret},
	})
}

// postTokenRequest sends a form-encoded POST to Discord's token endpoint
// and returns the parsed token response.
func (s *Server) postTokenRequest(data url.Values) (*TokenResponse, error) {
	resp, err := http.PostForm(s.oauthConfig.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			s.logger.Error("failed to close response body", "error", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	token := TokenResponse{}
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &token, nil
}
