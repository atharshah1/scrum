package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestNormalizeRequestedScopesRejectsUnknown(t *testing.T) {
	if _, err := normalizeRequestedScopes("read:issues unknown", []string{"read:issues"}); err == nil {
		t.Fatal("expected scope validation error")
	}
}

func TestValidatePKCE(t *testing.T) {
	verifier := "test-verifier"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if err := validatePKCE(verifier, challenge, "S256"); err != nil {
		t.Fatalf("expected PKCE validation to pass: %v", err)
	}
}

func TestValidateRedirectURI(t *testing.T) {
	if err := validateRedirectURI("http://127.0.0.1:8787/callback"); err != nil {
		t.Fatalf("expected loopback redirect to be allowed: %v", err)
	}
	if err := validateRedirectURI("http://example.com/callback"); err == nil {
		t.Fatal("expected non-https non-loopback redirect to be rejected")
	}
}
