package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestOrgContextMiddlewarePrefersExistingContextOverHeader(t *testing.T) {
	app := fiber.New()
	jwtOrgID := uuid.New()
	app.Get("/", func(c *fiber.Ctx) error {
		SetOrgID(c, jwtOrgID)
		return c.Next()
	}, OrgContextMiddleware(), func(c *fiber.Ctx) error {
		resolvedOrgID, ok := MustOrgID(c)
		if !ok {
			t.Fatal("expected org context to be available")
		}
		if resolvedOrgID != jwtOrgID {
			t.Fatalf("expected jwt org %s, got %s", jwtOrgID, resolvedOrgID)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Org-ID", uuid.NewString())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestOrgContextMiddlewareUsesHeaderAsFallback(t *testing.T) {
	app := fiber.New()
	expectedOrgID := uuid.New()
	app.Get("/", OrgContextMiddleware(), func(c *fiber.Ctx) error {
		resolvedOrgID, ok := MustOrgID(c)
		if !ok || resolvedOrgID != expectedOrgID {
			t.Fatalf("expected org header fallback to resolve %s", expectedOrgID)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Org-ID", expectedOrgID.String())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}
