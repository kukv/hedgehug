//go:build !e2e

package main

import (
	"os"
	"path/filepath"
)

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
