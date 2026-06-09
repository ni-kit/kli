package service

import (
	"fmt"
	"path/filepath"

	"github.com/ni-kit/kli/internal/domain"
	"github.com/ni-kit/kli/internal/repo"
)

type ToggleService interface {
	AddCommand(cwd string, inv domain.Invocation) (domain.Toggle, error)
	Get(cwd string) (*domain.Toggle, error)
	SetState(cwd string, state domain.ToggleState) error
}

type toggleService struct {
	repo repo.ToggleRepo
}

func NewToggleService(r repo.ToggleRepo) ToggleService {
	return &toggleService{repo: r}
}

func (s *toggleService) AddCommand(cwd string, inv domain.Invocation) (domain.Toggle, error) {
	cwd = cleanCwd(cwd)
	toggles, err := s.repo.LoadAll()
	if err != nil {
		return domain.Toggle{}, err
	}
	idx := -1
	for i := range toggles {
		if cleanCwd(toggles[i].Cwd) == cwd {
			idx = i
			break
		}
	}
	invCopy := inv
	if idx < 0 {
		next := domain.ToggleStateZero
		t := domain.Toggle{Cwd: cwd, One: &invCopy, State: domain.ToggleStateZero, NextAddState: &next}
		toggles = append(toggles, t)
		return t, s.repo.SaveAll(toggles)
	}

	t := toggles[idx]
	fp := inv.CommandFingerprint()
	switch {
	case t.One != nil && t.One.CommandFingerprint() == fp:
		t.One = &invCopy
	case t.Zero != nil && t.Zero.CommandFingerprint() == fp:
		t.Zero = &invCopy
	default:
		slot := t.NextAddSlot()
		if slot == domain.ToggleStateOne {
			t.One = &invCopy
		} else {
			t.Zero = &invCopy
		}
		next := domain.NextToggleState(slot)
		t.NextAddState = &next
	}
	toggles[idx] = t
	return t, s.repo.SaveAll(toggles)
}

func (s *toggleService) Get(cwd string) (*domain.Toggle, error) {
	cwd = cleanCwd(cwd)
	toggles, err := s.repo.LoadAll()
	if err != nil {
		return nil, err
	}
	for i := range toggles {
		if cleanCwd(toggles[i].Cwd) == cwd {
			return &toggles[i], nil
		}
	}
	return nil, fmt.Errorf("kli: no toggle configured for current directory")
}

func (s *toggleService) SetState(cwd string, state domain.ToggleState) error {
	cwd = cleanCwd(cwd)
	toggles, err := s.repo.LoadAll()
	if err != nil {
		return err
	}
	for i := range toggles {
		if cleanCwd(toggles[i].Cwd) == cwd {
			toggles[i].State = state
			return s.repo.SaveAll(toggles)
		}
	}
	return fmt.Errorf("kli: no toggle configured for current directory")
}

func cleanCwd(cwd string) string {
	if cwd == "" {
		return cwd
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}
	if eval, err := filepath.EvalSymlinks(cwd); err == nil {
		return eval
	}
	return filepath.Clean(cwd)
}
