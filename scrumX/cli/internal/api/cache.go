package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const readCacheTTL = 30 * time.Second

type cacheRecord struct {
	ExpiresAt time.Time       `json:"expires_at"`
	Data      json.RawMessage `json:"data"`
}

func (c *Client) cacheEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SCRUMX_DISABLE_CACHE"))) {
	case "1", "true", "yes", "on":
		return false
	default:
		return true
	}
}

func (c *Client) cacheScope() string {
	cfg, err := c.cfgStore.Load()
	if err != nil {
		return ""
	}
	orgID := strings.TrimSpace(cfg.CurrentOrgID)
	if orgID == "" {
		orgID = strings.TrimSpace(cfg.OrgID)
	}
	return strings.TrimSpace(cfg.APIURL) + "|" + orgID
}

func (c *Client) cacheDir() string {
	return filepath.Join(filepath.Dir(c.cfgStore.Path()), "cache")
}

func cacheHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (c *Client) cacheFile(prefix, key string) string {
	return filepath.Join(c.cacheDir(), prefix+"_"+cacheHash(key)+".json")
}

func (c *Client) cacheRead(prefix, key string, out any) bool {
	if !c.cacheEnabled() {
		return false
	}
	data, err := os.ReadFile(c.cacheFile(prefix, key))
	if err != nil {
		return false
	}
	var record cacheRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return false
	}
	if time.Now().After(record.ExpiresAt) {
		return false
	}
	return json.Unmarshal(record.Data, out) == nil
}

func (c *Client) cacheWrite(prefix, key string, value any) {
	if !c.cacheEnabled() {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	record := cacheRecord{
		ExpiresAt: time.Now().Add(readCacheTTL),
		Data:      encoded,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return
	}
	if err := os.MkdirAll(c.cacheDir(), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(c.cacheFile(prefix, key), payload, 0o600)
}

func (c *Client) invalidateIssueCache() {
	pattern := filepath.Join(c.cacheDir(), "issue*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, file := range files {
		_ = os.Remove(file)
	}
}
