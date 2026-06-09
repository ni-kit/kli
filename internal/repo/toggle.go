package repo

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ni-kit/kli/internal/domain"
)

type ToggleRepo interface {
	LoadAll() ([]domain.Toggle, error)
	SaveAll(toggles []domain.Toggle) error
}

type jsonToggleRepo struct {
	path string
}

func TogglePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kli", "toggles.json"), nil
}

func NewJSONToggleRepo(path string) ToggleRepo {
	return &jsonToggleRepo{path: path}
}

func (r *jsonToggleRepo) LoadAll() ([]domain.Toggle, error) {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var toggles []domain.Toggle
	if err := json.Unmarshal(data, &toggles); err != nil {
		return nil, err
	}
	return toggles, nil
}

func (r *jsonToggleRepo) SaveAll(toggles []domain.Toggle) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(toggles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}
