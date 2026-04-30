package repo

import (
	"os"
	"path/filepath"
)

func HistoryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kli", "history.json"), nil
}
