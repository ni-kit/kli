package service

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ni-kit/kli/internal/domain"
	"github.com/ni-kit/kli/internal/repo"
)

type ImportService interface {
	ImportShellHistory() (int, error)
}

type importService struct {
	histRepo repo.HistoryRepo
}

func NewImportService(r repo.HistoryRepo) ImportService {
	return &importService{histRepo: r}
}

func (s *importService) ImportShellHistory() (int, error) {
	var all []domain.Invocation
	for _, path := range historyFileCandidates() {
		invs, err := parseHistoryFile(path)
		if err != nil || len(invs) == 0 {
			continue
		}
		all = append(all, invs...)
	}

	existing, err := s.histRepo.Load()
	if err != nil {
		return 0, err
	}
	knownFPs := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		knownFPs[e.CommandFingerprint()] = struct{}{}
	}

	added := 0
	for _, inv := range all {
		fp := inv.CommandFingerprint()
		if _, known := knownFPs[fp]; !known {
			added++
			knownFPs[fp] = struct{}{} // prevent double-counting within all
		}
	}

	if err := s.histRepo.BulkAppend(all); err != nil {
		return 0, err
	}
	return added, nil
}

func historyFileCandidates() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".zsh_history"),
		filepath.Join(home, ".bash_history"),
		filepath.Join(home, ".sh_history"),
		filepath.Join(home, ".local", "share", "fish", "fish_history"),
		filepath.Join(home, ".history"),
		"/root/.zsh_history",
		"/root/.bash_history",
	}
}

func parseHistoryFile(path string) ([]domain.Invocation, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if strings.HasSuffix(path, "fish_history") {
		return parseFishHistory(f)
	}
	return parseLineHistory(f)
}

func parseLineHistory(f *os.File) ([]domain.Invocation, error) {
	var invs []domain.Invocation
	cwd, _ := os.Getwd()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var runAt time.Time
		cmd := line

		if strings.HasPrefix(line, ": ") {
			parts := strings.SplitN(line, ";", 2)
			if len(parts) == 2 {
				meta := strings.TrimPrefix(parts[0], ": ")
				tsParts := strings.SplitN(meta, ":", 2)
				if len(tsParts) == 2 {
					if ts, err := strconv.ParseInt(strings.TrimSpace(tsParts[0]), 10, 64); err == nil {
						runAt = time.Unix(ts, 0)
					}
				}
				cmd = parts[1]
			}
		}

		argv := domain.SplitShellLine(cmd)
		if len(argv) == 0 {
			continue
		}
		inv := parseWithTime(argv, runAt, cwd)
		invs = append(invs, inv)
	}
	return invs, scanner.Err()
}

func parseFishHistory(f *os.File) ([]domain.Invocation, error) {
	var invs []domain.Invocation
	var currentCmd string
	var currentTime time.Time
	cwd, _ := os.Getwd()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "- cmd: ") {
			if currentCmd != "" {
				if argv := domain.SplitShellLine(currentCmd); len(argv) > 0 {
					invs = append(invs, parseWithTime(argv, currentTime, cwd))
				}
			}
			currentCmd = strings.TrimPrefix(line, "- cmd: ")
			currentTime = time.Time{}
		} else if strings.HasPrefix(line, "  when: ") {
			ts, err := strconv.ParseInt(strings.TrimPrefix(line, "  when: "), 10, 64)
			if err == nil {
				currentTime = time.Unix(ts, 0)
			}
		}
	}
	if currentCmd != "" {
		if argv := domain.SplitShellLine(currentCmd); len(argv) > 0 {
			invs = append(invs, parseWithTime(argv, currentTime, cwd))
		}
	}
	return invs, scanner.Err()
}
