package middleware

import (
	"context"
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
		SetUserID(c, uuid.New())
		return c.Next()
	}, orgContextMiddleware(func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
		t.Fatal("membership checker should not run when org already exists")
		return false, nil
	}), func(c *fiber.Ctx) error {
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
	userID := uuid.New()
	expectedOrgID := uuid.New()
	app.Get("/", func(c *fiber.Ctx) error {
		SetUserID(c, userID)
		return c.Next()
	}, orgContextMiddleware(func(_ context.Context, gotUserID, gotOrgID uuid.UUID) (bool, error) {
		if gotUserID != userID {
			t.Fatalf("expected user %s, got %s", userID, gotUserID)
		}
		if gotOrgID != expectedOrgID {
			t.Fatalf("expected org %s, got %s", expectedOrgID, gotOrgID)
		}
		return true, nil
	}), func(c *fiber.Ctx) error {
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

func TestOrgContextMiddlewareRejectsHeaderForNonMember(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		SetUserID(c, uuid.New())
		return c.Next()
	}, orgContextMiddleware(func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
		return false, nil
	}), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Org-ID", uuid.NewString())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}
