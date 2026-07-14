package configstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const filename = "config.json"

type Config struct {
	GitHubOwner string `json:"github_owner,omitempty"`
}

type Store struct {
	path string
}

func New() (*Store, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config directory: %w", err)
	}

	return &Store{path: filepath.Join(root, "go-craft", filename)}, nil
}

func NewAt(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}

	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}

	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	cfg.GitHubOwner = strings.TrimSpace(cfg.GitHubOwner)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}

	if err := os.WriteFile(s.path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}

	return nil
}
