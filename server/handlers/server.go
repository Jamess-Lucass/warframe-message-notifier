package handlers

import (
	"github.com/bwmarrin/discordgo"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type Server struct {
	validator     *validator.Validate
	logger        *zap.Logger
	discordClient *discordgo.Session
}

func NewServer(logger *zap.Logger, discordClient *discordgo.Session) *Server {
	return &Server{
		validator:     validator.New(),
		logger:        logger,
		discordClient: discordClient,
	}
}
