package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Context struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type Config struct {
	CurrentContext string             `json:"current_context"`
	Contexts       map[string]Context `json:"contexts"`
}

func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jk", "config.json"), nil
}

func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	cfg := &Config{
		Contexts: make(map[string]Context),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if len(data) == 0 {
		return cfg, nil
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config json: %w", err)
	}
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]Context)
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func (c *Config) ResolveContext(target string) (string, *Context, error) {
	name := target
	if name == "" {
		name = os.Getenv("JK_CONTEXT")
	}
	if name == "" {
		name = c.CurrentContext
	}
	if name == "" {
		return "", nil, fmt.Errorf("no context selected (use 'jk context use <name>' or pass --context)")
	}

	ctx, ok := c.Contexts[name]
	if !ok {
		return "", nil, fmt.Errorf("context '%s' not found", name)
	}
	return name, &ctx, nil
}
