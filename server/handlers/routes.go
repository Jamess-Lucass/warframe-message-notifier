package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/discord/authorize", s.Authorize)
	mux.HandleFunc("GET /api/v1/discord/authorize/callback", s.AuthorizeCallback)

	mux.HandleFunc("POST /api/v1/oauth/exchange", s.ExchangeAuthCode)

	mux.HandleFunc("POST /api/v1/oauth/refresh", s.RefreshToken)

	// Send DM via bot
	mux.HandleFunc("POST /api/v1/discord/channels/@me/messages", s.SendChannelMessage)

	server := &http.Server{
		Addr:    ":8080",
		Handler: loggingMiddleware(s.logger, mux),
	}

	// Graceful shutdown: when the context is cancelled, shut down the server
	go func() {
		<-ctx.Done()
		s.logger.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("error shutting down server", "error", err)
		}
	}()

	s.logger.Info("Server listening on :8080")
	return server.ListenAndServe()
}
