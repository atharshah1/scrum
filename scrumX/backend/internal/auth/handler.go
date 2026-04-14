package auth

import (
	"github.com/atharshah1/scrum/scrumX/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service       *Service
	accessSecret  string
	refreshSecret string
}

func NewHandler(service *Service, accessSecret, refreshSecret string) *Handler {
	return &Handler{service: service, accessSecret: accessSecret, refreshSecret: refreshSecret}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	auth := api.Group("/auth")
	auth.Post("/register", h.register)
	auth.Post("/login", h.login)
	auth.Post("/refresh", h.refresh)
	auth.Get("/me", h.me)
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(c *fiber.Ctx) error {
	var req credentials
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	user, tokens, err := h.service.Register(c.Context(), req.Email, req.Password)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusCreated, fiber.Map{"user": user, "tokens": tokens})
}

func (h *Handler) login(c *fiber.Ctx) error {
	var req credentials
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	user, tokens, err := h.service.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"user": user, "tokens": tokens})
}

func (h *Handler) refresh(c *fiber.Ctx) error {
	var payload struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "invalid payload")
	}
	claims, err := ParseRefreshClaims(payload.RefreshToken, h.refreshSecret)
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}
	tokens, err := h.service.Refresh(c.Context(), claims.UserID, claims.OrgID, claims.Role, claims.TokenID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}
	return utils.JSONSuccess(c, fiber.StatusOK, tokens)
}

func (h *Handler) me(c *fiber.Ctx) error {
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"message": "use /auth/login and /auth/register; /auth/me can be wired to user store"})
}
