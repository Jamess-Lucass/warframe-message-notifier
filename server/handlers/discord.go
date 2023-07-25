package handlers

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

// https://discord.com/developers/docs/resources/channel#create-message-jsonform-params
type SendChannelMessageRequest struct {
	Content string `json:"content" validate:"required,max=2000"`
}

var oauthConfig = &oauth2.Config{
	ClientID:     os.Getenv("DISCORD_BOT_CLIENT_ID"),
	ClientSecret: os.Getenv("DISCORD_BOT_CLIENT_SECRET"),
	RedirectURL:  os.Getenv("DISCORD_BOT_REDIRECT_URI"),
	Scopes:       []string{"identify"},
	Endpoint: oauth2.Endpoint{
		AuthURL:   "https://discord.com/api/oauth2/authorize",
		TokenURL:  "https://discord.com/api/oauth2/token",
		AuthStyle: oauth2.AuthStyleInParams,
	},
}

func (s *Server) Authorize(c *fiber.Ctx) error {
	uri, err := url.Parse(oauthConfig.Endpoint.AuthURL)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: "Could not parse url"})
	}

	params := url.Values{}
	params.Add("client_id", oauthConfig.ClientID)
	params.Add("scope", strings.Join(oauthConfig.Scopes, " "))
	params.Add("redirect_uri", oauthConfig.RedirectURL)
	params.Add("response_type", "code")
	uri.RawQuery = params.Encode()

	return c.Status(fiber.StatusTemporaryRedirect).Redirect(uri.String())
}

func (s *Server) AuthorizeCallBack(c *fiber.Ctx) error {
	code := c.Query("code")
	token, err := oauthConfig.Exchange(c.Context(), code)
	if err != nil {
		s.logger.Sugar().Errorf("error occured while exchanging auth code for token: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: "Failed to obtain token"})
	}

	uri := fmt.Sprintf("%s/api/v1/oauth/token?token=%s", os.Getenv("CLIENT_API_BASE_URL"), token.AccessToken)

	return c.Status(fiber.StatusPermanentRedirect).Redirect(uri)
}

func (s *Server) SendChannelMessage(c *fiber.Ctx) error {
	var request SendChannelMessageRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: err.Error()})
	}

	if err := s.validator.Struct(request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Message: err.Error()})
	}

	client, err := discordgo.New(c.Get("Authorization"))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Message: err.Error()})
	}

	user, err := client.User("@me")
	if err != nil {
		s.logger.Sugar().Errorf("error occured while fetching user details: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: err.Error()})
	}

	ch, err := s.discordClient.UserChannelCreate(user.ID)
	if err != nil {
		s.logger.Sugar().Errorf("error occured while fetching DM channel: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: err.Error()})
	}

	msg, err := s.discordClient.ChannelMessageSend(ch.ID, request.Content)
	if err != nil {
		s.logger.Sugar().Errorf("error occured while sending DM: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: err.Error()})
	}

	s.logger.Sugar().Infof("Sent Discord message with content: %s", msg.Content)

	return c.SendStatus(fiber.StatusNoContent)
}
