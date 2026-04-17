package integrations

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type credentialCipher struct {
	key []byte
}

type credentialEnvelope struct {
	Version    string   `json:"version"`
	Algorithm  string   `json:"algorithm"`
	Nonce      string   `json:"nonce"`
	Ciphertext string   `json:"ciphertext"`
	Keys       []string `json:"keys"`
}

func newCredentialCipher(rawKey string) (*credentialCipher, error) {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return nil, errors.New("integration credentials key is required (set INTEGRATION_CREDENTIALS_KEY)")
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(key)
		if err != nil {
			return nil, errors.New("integration credentials key must be base64-encoded")
		}
	}
	if len(decoded) != 32 {
		return nil, errors.New("integration credentials key must decode to 32 bytes")
	}
	return &credentialCipher{key: decoded}, nil
}

func (c *credentialCipher) Encrypt(plaintext []byte, keys []string) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	envelope := credentialEnvelope{
		Version:    "v1",
		Algorithm:  "AES-256-GCM",
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Keys:       keys,
	}
	return json.Marshal(envelope)
}

func (c *credentialCipher) Decrypt(raw []byte) ([]byte, []string, error) {
	envelope := credentialEnvelope{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, nil, err
	}
	if envelope.Ciphertext == "" || envelope.Nonce == "" || envelope.Algorithm == "" {
		return nil, nil, errors.New("invalid or corrupted credential envelope")
	}
	if envelope.Algorithm != "AES-256-GCM" {
		return nil, nil, errors.New("unsupported credential encryption algorithm")
	}
	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, nil, err
	}
	keys := envelope.Keys
	if len(keys) == 0 {
		decoded := map[string]any{}
		if err := json.Unmarshal(plaintext, &decoded); err != nil {
			return nil, nil, err
		}
		keys = mapKeys(decoded)
	}
	return plaintext, keys, nil
}
