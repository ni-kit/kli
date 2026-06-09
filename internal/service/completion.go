package service

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

type CompletionOptions struct {
	Value       string
	LocalEnv    []string
	Cwd         string
	EnableEnv   bool
	EnableFiles bool
}

const MaxCompletionSuggestions = 3

type CompletionSuggestion struct {
	Value  string
	Suffix string
}

func CompleteInput(opts CompletionOptions) string {
	suggestions := CompleteInputs(opts)
	if len(suggestions) == 0 {
		return ""
	}
	return suggestions[0].Suffix
}

func CompleteInputs(opts CompletionOptions) []CompletionSuggestion {
	if opts.EnableEnv {
		if suggestions := envCompletions(opts.Value, availableEnvNames(opts.LocalEnv)); len(suggestions) > 0 {
			return suggestions
		}
	}
	if opts.EnableFiles {
		return fileCompletions(opts.Value, opts.Cwd)
	}
	return nil
}

func availableEnvNames(local []string) []string {
	seen := map[string]struct{}{}
	var names []string
	for _, name := range local {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	sysNames := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok && k != "" {
			sysNames = append(sysNames, k)
		}
	}
	slices.Sort(sysNames)
	for _, name := range sysNames {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func envCompletions(value string, names []string) []CompletionSuggestion {
	if braceIdx := strings.LastIndex(value, "${"); braceIdx >= 0 {
		prefix := value[braceIdx+2:]
		if prefix != "" && !strings.Contains(prefix, "}") && isEnvIdentifier(prefix) {
			var suggestions []CompletionSuggestion
			for _, name := range names {
				if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
					suggestions = append(suggestions, CompletionSuggestion{
						Value:  "${" + name + "}",
						Suffix: name[len(prefix):] + "}",
					})
					if len(suggestions) == MaxCompletionSuggestions {
						break
					}
				}
			}
			return suggestions
		}
	}

	if idx := strings.LastIndex(value, "$"); idx >= 0 {
		prefix := value[idx+1:]
		if prefix != "" && isEnvIdentifier(prefix) {
			var suggestions []CompletionSuggestion
			for _, name := range names {
				if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
					suggestions = append(suggestions, CompletionSuggestion{
						Value:  "$" + name,
						Suffix: name[len(prefix):],
					})
					if len(suggestions) == MaxCompletionSuggestions {
						break
					}
				}
			}
			return suggestions
		}
	}
	return nil
}

func isEnvIdentifier(s string) bool {
	for _, r := range s {
		if r != '_' && !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func fileCompletions(value, cwd string) []CompletionSuggestion {
	token := lastToken(value)
	pathToken := strings.TrimPrefix(strings.TrimPrefix(token, ">>"), ">")
	if strings.ContainsAny(pathToken, "\"'`") {
		return nil
	}

	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	dirPart, base := filepath.Split(pathToken)
	searchDir := completionSearchDir(dirPart, cwd)
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		if !strings.HasPrefix(name, base) {
			continue
		}
		if entry.IsDir() {
			name += string(filepath.Separator)
		}
		matches = append(matches, name)
	}
	if len(matches) == 0 {
		return nil
	}
	slices.Sort(matches)
	n := min(len(matches), MaxCompletionSuggestions)
	suggestions := make([]CompletionSuggestion, 0, n)
	for _, match := range matches[:n] {
		suggestions = append(suggestions, CompletionSuggestion{
			Value:  dirPart + match,
			Suffix: match[len(base):],
		})
	}
	return suggestions
}

func lastToken(value string) string {
	value = strings.TrimRightFunc(value, unicode.IsSpace)
	idx := strings.LastIndexFunc(value, unicode.IsSpace)
	if idx < 0 {
		return value
	}
	return value[idx+1:]
}

func completionSearchDir(dirPart, cwd string) string {
	switch {
	case dirPart == "":
		if cwd != "" {
			return cwd
		}
		return "."
	case strings.HasPrefix(dirPart, "~/"):
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(dirPart, "~/"))
		}
	case filepath.IsAbs(dirPart):
		return dirPart
	}
	if cwd == "" {
		return filepath.Clean(dirPart)
	}
	return filepath.Join(cwd, dirPart)
}
