package repo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/ni-kit/kli/internal/domain"
)

type HistoryRepo interface {
	Load() ([]domain.Invocation, error)
	Append(inv domain.Invocation) error
	BulkAppend(invs []domain.Invocation) error
	UpdateAll(invs []domain.Invocation) error
}

type jsonHistoryRepo struct {
	path string
}

func NewJSONHistoryRepo(path string) HistoryRepo {
	return &jsonHistoryRepo{path: path}
}

func (r *jsonHistoryRepo) Load() ([]domain.Invocation, error) {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var invs []domain.Invocation
	if err := json.Unmarshal(data, &invs); err != nil {
		return nil, err
	}

	invs = dedup(invs)
	slices.SortFunc(invs, func(a, b domain.Invocation) int {
		return b.LastRun().RunAt.Compare(a.LastRun().RunAt)
	})
	return invs, nil
}

func (r *jsonHistoryRepo) Append(inv domain.Invocation) error {
	if len(inv.Runs) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}

	existing, err := r.Load()
	if err != nil {
		return err
	}

	newRun := inv.Runs[0]
	fp := inv.CommandFingerprint()
	var found bool
	for i := range existing {
		if existing[i].CommandFingerprint() == fp {
			existing[i].AddRun(newRun)
			existing[i].Args = inv.Args
			found = true
			break
		}
	}
	if !found {
		existing = append([]domain.Invocation{inv}, existing...)
	}

	slices.SortFunc(existing, func(a, b domain.Invocation) int {
		return b.LastRun().RunAt.Compare(a.LastRun().RunAt)
	})
	return r.write(existing)
}

func (r *jsonHistoryRepo) BulkAppend(incoming []domain.Invocation) error {
	if len(incoming) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}

	existing, err := r.Load()
	if err != nil {
		return err
	}

	index := make(map[string]int, len(existing))
	for i, e := range existing {
		index[e.CommandFingerprint()] = i
	}

	for _, inv := range incoming {
		if len(inv.Runs) == 0 {
			continue
		}
		fp := inv.CommandFingerprint()
		if idx, ok := index[fp]; ok {
			existing[idx].AddRun(inv.Runs[0])
			existing[idx].Args = inv.Args
		} else {
			index[fp] = len(existing)
			existing = append(existing, inv)
		}
	}

	slices.SortFunc(existing, func(a, b domain.Invocation) int {
		return b.LastRun().RunAt.Compare(a.LastRun().RunAt)
	})
	return r.write(existing)
}

func (r *jsonHistoryRepo) UpdateAll(invs []domain.Invocation) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	return r.write(invs)
}

func (r *jsonHistoryRepo) write(invs []domain.Invocation) error {
	data, err := json.MarshalIndent(invs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}

func dedup(invs []domain.Invocation) []domain.Invocation {
	seen := map[string]int{}
	result := invs[:0:0]
	for _, inv := range invs {
		fp := inv.CommandFingerprint()
		if idx, ok := seen[fp]; ok {
			result[idx].Runs = append(result[idx].Runs, inv.Runs...)
			if len(inv.Runs) > 0 && (len(result[idx].Runs) == 0 || inv.Runs[0].RunAt.After(result[idx].Runs[0].RunAt)) {
				result[idx].Args = inv.Args
			}
		} else {
			seen[fp] = len(result)
			result = append(result, inv)
		}
	}
	for i := range result {
		slices.SortFunc(result[i].Runs, func(a, b domain.Run) int {
			return b.RunAt.Compare(a.RunAt)
		})
	}
	return result
}
