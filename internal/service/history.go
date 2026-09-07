package service

import (
	"fmt"

	"github.com/ni-kit/kli/internal/domain"
	"github.com/ni-kit/kli/internal/repo"
)

type HistoryService interface {
	All() ([]domain.Invocation, error)
	Record(inv domain.Invocation) error
	SetTags(invID, fingerprint string, tags []string) error
	SaveLayout(invID string, env []domain.EnvVar, args []domain.Arg) error
	SaveEdited(orig, updated domain.Invocation) error
	Delete(invID, fingerprint string) error
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

func (s *historyService) mutateByRef(id, fingerprint string, fn func(*domain.Invocation)) error {
	invs, err := s.repo.Load()
	if err != nil {
		return err
	}
	for i := range invs {
		if invs[i].ID == id && (fingerprint == "" || invs[i].CommandFingerprint() == fingerprint) {
			fn(&invs[i])
			return s.repo.UpdateAll(invs)
		}
	}
	return fmt.Errorf("invocation %q not found", id)
}

func (s *historyService) SetTags(invID, fingerprint string, tags []string) error {
	return s.mutateByRef(invID, fingerprint, func(inv *domain.Invocation) { inv.Tags = tags })
}

func (s *historyService) Delete(invID, fingerprint string) error {
	invs, err := s.repo.Load()
	if err != nil {
		return err
	}
	filtered := invs[:0]
	for _, inv := range invs {
		if inv.ID != invID || (fingerprint != "" && inv.CommandFingerprint() != fingerprint) {
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

	// Inside a tmux/zellij pane, auto-scope to the last command run in THIS pane.
	if scope := currentPaneScope(); scope != nil {
		if best, ok := latestByRun(invs, func(inv domain.Invocation) (domain.Run, bool) {
			return inv.LastRunWithMetaAll(scope)
		}); ok {
			return best, nil
		}
		// No history for this pane; fall through to dir/global behavior.
	}

	if !currentDirOnly {
		return &invs[0], nil
	}
	best, ok := latestByRun(invs, func(inv domain.Invocation) (domain.Run, bool) {
		return inv.LastRunInDir(cwd)
	})
	if !ok {
		return nil, fmt.Errorf("kli: no history for current directory")
	}
	return best, nil
}

// latestByRun returns the invocation whose matching run (per pick) is newest.
func latestByRun(invs []domain.Invocation, pick func(domain.Invocation) (domain.Run, bool)) (*domain.Invocation, bool) {
	var best *domain.Invocation
	var bestRun domain.Run
	for i := range invs {
		run, ok := pick(invs[i])
		if !ok {
			continue
		}
		if best == nil || run.RunAt.After(bestRun.RunAt) {
			best = &invs[i]
			bestRun = run
		}
	}
	return best, best != nil
}

func (s *historyService) SaveLayout(invID string, env []domain.EnvVar, args []domain.Arg) error {
	return s.mutateByRef(invID, "", func(inv *domain.Invocation) { inv.Env = env; inv.Args = args })
}

func (s *historyService) SaveEdited(orig, updated domain.Invocation) error {
	if updated.CommandFingerprint() == orig.CommandFingerprint() {
		return s.mutateByRef(orig.ID, orig.CommandFingerprint(), func(inv *domain.Invocation) {
			inv.Env = updated.Env
			inv.Args = updated.Args
			inv.Stdout = updated.Stdout
			inv.Stderr = updated.Stderr
			inv.Chain = updated.Chain
			inv.Tags = updated.Tags
		})
	}
	updated.ID = newID()
	return s.Record(updated)
}
