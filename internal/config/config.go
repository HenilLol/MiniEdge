package config

import (
	"encoding/json"
	"fmt"
	"io"

	"miniedge/internal/model"
)

// ParseConfig decodes JSON data into a model.Config and validates it.
func ParseConfig(data []byte) (*model.Config, error) {
	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON config: %w", err)
	}
	if err := ValidateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadConfig reads JSON configuration from an io.Reader and validates it.
func LoadConfig(r io.Reader) (*model.Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config stream: %w", err)
	}
	return ParseConfig(data)
}
