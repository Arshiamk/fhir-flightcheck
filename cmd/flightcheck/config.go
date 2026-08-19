package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfigPath = ".flightcheck.json"

type config struct {
	APIURL    string `json:"apiUrl"`
	ProjectID string `json:"projectId"`
	TargetID  string `json:"targetId,omitempty"`
	LastRunID string `json:"lastRunId,omitempty"`
	PublicKey string `json:"publicKey"`
	KeyID     string `json:"keyId"`
}

func loadConfig(path string) (config, error) {
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config{}, fmt.Errorf("configuration %q does not exist; run flightcheck init", path)
		}
		return config{}, fmt.Errorf("read configuration: %w", err)
	}
	var value config
	if err := json.Unmarshal(body, &value); err != nil {
		return config{}, fmt.Errorf("parse configuration: %w", err)
	}
	if value.APIURL == "" || value.ProjectID == "" {
		return config{}, errors.New("configuration is missing apiUrl or projectId")
	}
	return value, nil
}

func saveConfig(path string, value config) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal configuration: %w", err)
	}
	body = append(body, '\n')
	cleanPath := filepath.Clean(path)
	temporary, err := os.CreateTemp(filepath.Dir(cleanPath), ".flightcheck-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary configuration: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary configuration: %w", err)
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary configuration: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary configuration: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary configuration: %w", err)
	}
	if err := os.Rename(temporaryPath, cleanPath); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}
	return nil
}
