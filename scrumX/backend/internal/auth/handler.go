package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}
	user, tokens, err := h.service.Register(c.Context(), req.Email, req.Password)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": user, "tokens": tokens})
}

func (h *Handler) login(c *fiber.Ctx) error {
	var req credentials
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}
	user, tokens, err := h.service.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"user": user, "tokens": tokens})
}

func (h *Handler) refresh(c *fiber.Ctx) error {
	var payload struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}
	token, err := jwt.Parse(payload.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "unexpected signing method")
		}
		return []byte(h.refreshSecret), nil
	})
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid refresh token"})
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token claims"})
	}
	subject, ok := claims["sub"].(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid subject claim"})
	}
	userID, err := uuid.Parse(subject)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid subject claim"})
	}
	user, err := h.service.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unknown user"})
	}
	tokens, err := GenerateTokens(userID, user.OrgID, user.Role, h.accessSecret, h.refreshSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to mint tokens"})
	}
	return c.JSON(tokens)
}

func (h *Handler) me(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "use /auth/login and /auth/register; /auth/me can be wired to user store"})
}
