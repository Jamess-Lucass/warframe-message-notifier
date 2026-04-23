package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"
)

type SendChannelMessageRequest struct {
	Content string `json:"content"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type tokenState struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func newTokenState(resp *TokenResponse) *tokenState {
	return &tokenState{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}
}

func (t *tokenState) needsRefresh() bool {
	return time.Until(t.ExpiresAt) < 60*time.Second
}

var apiBaseUrl string

// Regex for detecting the event when a tab is added to the chat window.
// DE prepends 'F' to the username for player direct messages.
var chatTabRegex = regexp.MustCompile(`(Script \[Info\]: ChatRedux\.lua: ChatRedux::AddTab: Adding tab with channel name: F)(.+)( to index.+)`)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.Println("Starting!")

	// When using 'go build' we inject apiBaseUrl via -ldflags -X.
	// When using Docker, we read from the environment variable.
	if apiBaseUrl == "" {
		apiBaseUrl = os.Getenv("API_BASE_URL")
	}

	eeLogPath := os.Getenv("WF_EE_LOG_FILE_PATH")
	if eeLogPath == "" {
		return fmt.Errorf("environment variable 'WF_EE_LOG_FILE_PATH' is not set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tokenCh := make(chan *TokenResponse, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code parameter", http.StatusBadRequest)
			return
		}

		token, err := exchangeAuthCode(code)
		if err != nil {
			log.Printf("failed to exchange auth code: %v", err)
			http.Error(w, "Failed to exchange auth code", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Successful! You may close this tab and navigate back to your command line.")); err != nil {
			log.Printf("failed to write response: %v", err)
		}

		select {
		case tokenCh <- token:
		default:
		}
	})

	server := &http.Server{Addr: ":8081", Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Replace 'host.docker.internal' with 'localhost' for display only --
	// the user accesses this from the host machine, not inside the container.
	displayURL := strings.ReplaceAll(apiBaseUrl, "host.docker.internal", "localhost")
	log.Printf("Please authenticate with Discord via: %s/api/v1/discord/authorize", displayURL)

	// Block until we receive a token or the context is cancelled.
	var token *tokenState
	select {
	case resp := <-tokenCh:
		token = newTokenState(resp)
		log.Println("Successfully authenticated with Discord.")
	case <-ctx.Done():
		log.Println("Shutting down before authentication completed.")
		shutdownServer(server)
		return nil
	}

	file, err := os.Open(eeLogPath)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("error closing file: %v", err)
		}
	}()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("error seeking to end of file: %w", err)
	}

	reader := bufio.NewReader(file)
	log.Println("Watching log file for incoming messages...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down...")
			shutdownServer(server)
			return nil
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(1 * time.Second)
				continue
			}
			return fmt.Errorf("error reading line: %w", err)
		}

		if chatTabRegex.MatchString(line) {
			matches := chatTabRegex.FindStringSubmatch(line)
			username := removeNonPrintableCharacters(matches[2])
			log.Printf("Received DM from %s", username)

			if token.needsRefresh() {
				log.Println("Access token expiring soon, refreshing...")
				resp, err := refreshToken(token.RefreshToken)
				if err != nil {
					log.Printf("failed to refresh token: %v", err)
					continue
				}
				token = newTokenState(resp)
			}

			if err := sendDiscordMessage(token.AccessToken, username); err != nil {
				log.Printf("error sending Discord message: %v", err)
			}
		}
	}
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("error shutting down HTTP server: %v", err)
	}
}

func postJSON(url string, body any) (*TokenResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errResp := ErrorResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("request failed: %s", errResp.Message)
	}

	tokenResp := TokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &tokenResp, nil
}

func exchangeAuthCode(code string) (*TokenResponse, error) {
	url := fmt.Sprintf("%s/api/v1/oauth/exchange", apiBaseUrl)
	return postJSON(url, map[string]string{"code": code})
}

func refreshToken(refreshTok string) (*TokenResponse, error) {
	url := fmt.Sprintf("%s/api/v1/oauth/refresh", apiBaseUrl)
	return postJSON(url, map[string]string{"refresh_token": refreshTok})
}

func removeNonPrintableCharacters(val string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, val)
}

func sendDiscordMessage(accessToken string, username string) error {
	content := SendChannelMessageRequest{
		Content: fmt.Sprintf("You received a new direct message from __**%s**__", username),
	}

	body, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/discord/channels/@me/messages", apiBaseUrl)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}()

	if resp.StatusCode >= 400 {
		errResp := ErrorResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("failed to send discord message: %s. Please ensure you have at least one mutual server with the Discord Bot", errResp.Message)
	}

	return nil
}
