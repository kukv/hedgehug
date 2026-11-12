//go:build !e2e

package main

import (
	"os"
	"path/filepath"
)

// statePath は state.db の場所(保存形式「このマシンに持つもの」)。XDG_DATA_HOME があればその下、
// なければ ~/.local/share の下。
func statePath() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "hedgehug", "state.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "hedgehug", "state.db"), nil
}

// configPath は config.yaml の場所(保存形式「このマシンに持つもの」)。XDG_CONFIG_HOME があればその下、
// なければ ~/.config の下。
func configPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "hedgehug", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "hedgehug", "config.yaml"), nil
}
