package handlers

import (
	"github.com/gofiber/contrib/fiberzap"
	"github.com/gofiber/fiber/v2"
)

func (s *Server) Start() error {
	f := fiber.New()

	f.Use(fiberzap.New(fiberzap.Config{
		Logger: s.logger,
	}))

	f.Post("/api/v1/discord/channels/@me/messages", s.SendChannelMessage)
	f.Get("/api/v1/discord/authorize", s.Authorize)
	f.Get("/api/v1/discord/authorize/callback", s.AuthorizeCallBack)

	f.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Message: "No resource found"})
	})

	return f.Listen(":8080")
}
