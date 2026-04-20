package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const defaultAPIURL = "http://localhost:8080/api/v1"

type Config struct {
	APIURL           string `mapstructure:"api_url"`
	AccessToken      string `mapstructure:"access_token"`
	RefreshToken     string `mapstructure:"refresh_token"`
	OAuthClientID    string `mapstructure:"oauth_client_id"`
	OAuthScope       string `mapstructure:"oauth_scope"`
	OrgID            string `mapstructure:"org_id"`
	UserID           string `mapstructure:"user_id"`
	CurrentOrgID     string `mapstructure:"current_org_id"`
	CurrentProjectID string `mapstructure:"current_project_id"`
	CurrentBoardID   string `mapstructure:"current_board_id"`
}

type Store struct {
	path string
}

func NewStore(path string) *Store {
	if path == "" {
		path = defaultPath()
	}
	return &Store{path: path}
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (Config, error) {
	cfg := Config{APIURL: defaultAPIURL}
	v := viper.New()
	v.SetConfigFile(s.path)
	v.SetConfigType("yaml")
	v.SetDefault("api_url", defaultAPIURL)

	if err := v.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.APIURL == "" {
		cfg.APIURL = defaultAPIURL
	}
	if cfg.CurrentOrgID == "" {
		cfg.CurrentOrgID = cfg.OrgID
	}
	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	if cfg.APIURL == "" {
		cfg.APIURL = defaultAPIURL
	}
	if cfg.CurrentOrgID == "" {
		cfg.CurrentOrgID = cfg.OrgID
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.Set("api_url", cfg.APIURL)
	v.Set("access_token", cfg.AccessToken)
	v.Set("refresh_token", cfg.RefreshToken)
	v.Set("oauth_client_id", cfg.OAuthClientID)
	v.Set("oauth_scope", cfg.OAuthScope)
	v.Set("org_id", cfg.OrgID)
	v.Set("user_id", cfg.UserID)
	v.Set("current_org_id", cfg.CurrentOrgID)
	v.Set("current_project_id", cfg.CurrentProjectID)
	v.Set("current_board_id", cfg.CurrentBoardID)

	if err := v.WriteConfigAs(s.path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}
	return nil
}

func defaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".scrumx/config.yaml"
	}
	return filepath.Join(home, ".scrumx", "config.yaml")
}
