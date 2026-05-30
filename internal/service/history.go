package service

import (
	"fmt"

	"github.com/ni-kit/kli/internal/domain"
	"github.com/ni-kit/kli/internal/repo"
)

type HistoryService interface {
	All() ([]domain.Invocation, error)
	Record(inv domain.Invocation) error
	SetTags(invID string, tags []string) error
	SaveLayout(invID string, env []domain.EnvVar, args []domain.Arg) error
	SaveEdited(orig, updated domain.Invocation) error
	Delete(invID string) error
	Latest(cwd string, currentDirOnly bool) (*domain.Invocation, error)
}

type historyService struct {
	repo repo.HistoryRepo
}

func NewHistoryService(r repo.HistoryRepo) HistoryService {
	return &historyService{repo: r}
}

func (s *historyService) All() ([]domain.Invocation, error) {
	return s.repo.Load()
}

func (s *historyService) Record(inv domain.Invocation) error {
	return s.repo.Append(inv)
}

func (s *historyService) mutateByID(id string, fn func(*domain.Invocation)) error {
	invs, err := s.repo.Load()
	if err != nil {
		return err
	}
	for i := range invs {
		if invs[i].ID == id {
			fn(&invs[i])
			return s.repo.UpdateAll(invs)
		}
	}
	return fmt.Errorf("invocation %q not found", id)
}

func (s *historyService) SetTags(invID string, tags []string) error {
	return s.mutateByID(invID, func(inv *domain.Invocation) { inv.Tags = tags })
}

func (s *historyService) Delete(invID string) error {
	invs, err := s.repo.Load()
	if err != nil {
		return err
	}
	filtered := invs[:0]
	for _, inv := range invs {
		if inv.ID != invID {
			filtered = append(filtered, inv)
		}
	}
	return s.repo.UpdateAll(filtered)
}

func (s *historyService) Latest(cwd string, currentDirOnly bool) (*domain.Invocation, error) {
	invs, err := s.repo.Load()
	if err != nil {
		return nil, err
	}
	if len(invs) == 0 {
		return nil, fmt.Errorf("kli: no history yet")
	}
	if !currentDirOnly {
		return &invs[0], nil
	}
	var best *domain.Invocation
	var bestRun domain.Run
	for i := range invs {
		run, ok := invs[i].LastRunInDir(cwd)
		if !ok {
			continue
		}
		if best == nil || run.RunAt.After(bestRun.RunAt) {
			best = &invs[i]
			bestRun = run
		}
	}
	if best == nil {
		return nil, fmt.Errorf("kli: no history for current directory")
	}
	return best, nil
}

func (s *historyService) SaveLayout(invID string, env []domain.EnvVar, args []domain.Arg) error {
	return s.mutateByID(invID, func(inv *domain.Invocation) { inv.Env = env; inv.Args = args })
}

func (s *historyService) SaveEdited(orig, updated domain.Invocation) error {
	if updated.CommandFingerprint() == orig.CommandFingerprint() {
		return s.SaveLayout(orig.ID, updated.Env, updated.Args)
	}
	return s.Record(updated)
}
