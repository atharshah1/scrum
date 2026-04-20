package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequireScopesAllowsLegacyTokens(t *testing.T) {
	app := fiber.New()
	app.Get("/", RequireScopes("read:issues"), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected legacy token path to pass, got %d", resp.StatusCode)
	}
}

func TestRequireScopesRejectsMissingOAuthScope(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		SetClientID(c, "scrumx-cli")
		SetScopes(c, []string{"read:projects"})
		return c.Next()
	}, RequireScopes("read:issues"), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", resp.StatusCode)
	}
}
