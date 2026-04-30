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

func (s *historyService) SetTags(invID string, tags []string) error {
	invs, err := s.repo.Load()
	if err != nil {
		return err
	}
	for i := range invs {
		if invs[i].ID == invID {
			invs[i].Tags = tags
			return s.repo.UpdateAll(invs)
		}
	}
	return fmt.Errorf("invocation %q not found", invID)
}
