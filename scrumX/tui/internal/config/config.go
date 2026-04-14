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
	OrgID            string `mapstructure:"org_id"`
	CurrentOrgID     string `mapstructure:"current_org_id"`
	CurrentProjectID string `mapstructure:"current_project_id"`
	CurrentBoardID   string `mapstructure:"current_board_id"`
}

func Load() (Config, error) {
	cfg := Config{APIURL: defaultAPIURL}
	v := viper.New()
	v.SetConfigFile(defaultPath())
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

func defaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".scrumx/config.yaml"
	}
	return filepath.Join(home, ".scrumx", "config.yaml")
}
