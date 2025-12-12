package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ClientID     string
	ClientSecret string
	APIUrl       string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()
	clientID := os.Getenv("PALABRA_CLIENT_ID")
	clientSecret := os.Getenv("PALABRA_CLIENT_SECRET")
	if clientID == "" {
		return nil, fmt.Errorf("PALABRA_CLIENT_ID is required")
	}

	if clientSecret == "" {
		return nil, fmt.Errorf("PALABRA_CLIENT_SECRET is required")
	}
	apiURL := os.Getenv("PALABRA_API_URL")
	if apiURL == "" {
		apiURL = "https://api.palabra.dev/session-storage/session"
	}

	return &Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		APIUrl:       apiURL,
	}, nil
}

func (c *Config) Validate() error {
	if c.ClientID == "" {
		return fmt.Errorf("ClientID cannot be empty")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("ClientSecret cannot be empty")
	}
	if c.APIUrl == "" {
		return fmt.Errorf("APIUrl cannot be empty")
	}
	return nil
}
