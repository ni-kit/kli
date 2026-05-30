package service

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ni-kit/kli/internal/domain"
)

func Parse(argv []string) domain.Invocation {
	cwd, _ := os.Getwd()
	return parseWithTime(argv, time.Now(), cwd)
}

func parseWithTime(argv []string, t time.Time, cwd string) domain.Invocation {
	env, argv := splitLeadingEnv(argv)
	tokens, stdout, stderr := extractRedirects(argv[1:])
	inv := domain.Invocation{
		ID:      newID(),
		Command: argv[0],
		Env:     env,
		Stdout:  stdout,
		Stderr:  stderr,
		Runs:    []domain.Run{{RunAt: t, Cwd: cwd}},
	}

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case strings.HasPrefix(tok, "--"):
			name := tok[2:]
			if idx := strings.IndexByte(name, '='); idx >= 0 {
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgLongFlag, Name: name[:idx], Value: name[idx+1:]})
			} else if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgLongFlag, Name: name})
				i++
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgFlagValue, Value: tokens[i]})
			} else {
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgLongFlag, Name: name})
			}

		case strings.HasPrefix(tok, "-") && len(tok) > 1:
			letters := tok[1:]
			for j, ch := range letters {
				inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgShortFlag, Name: string(ch)})
				if j == len(letters)-1 && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
					i++
					inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgFlagValue, Value: tokens[i]})
				}
			}

		default:
			inv.Args = append(inv.Args, domain.Arg{Kind: domain.ArgPositional, Value: tok})
		}
	}
	return inv
}

func splitLeadingEnv(argv []string) ([]domain.EnvVar, []string) {
	var env []domain.EnvVar
	for len(argv) > 1 {
		key, value, ok := strings.Cut(argv[0], "=")
		if !ok || key == "" || !isEnvKey(key) {
			break
		}
		env = append(env, domain.EnvVar{Key: key, Value: value})
		argv = argv[1:]
	}
	return env, argv
}

func isEnvKey(s string) bool {
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func newID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// extractRedirects scans tokens for shell redirect operators, removes them from
// the list, and returns the cleaned tokens plus stdout/stderr redirect config.
func extractRedirects(tokens []string) (remaining []string, stdout, stderr domain.StreamRedirect) {
	i := 0
	for i < len(tokens) {
		tok := tokens[i]

		switch tok {
		case "2>&1":
			stderr = domain.StreamRedirect{Target: domain.RedirectToOther}
			i++
			continue
		case ">&2", "1>&2":
			stdout = domain.StreamRedirect{Target: domain.RedirectToOther}
			i++
			continue
		}

		// Two-token operators: ">" "file", ">>" "file", "2>" "file", etc.
		if op, isStdout, ok := standaloneRedirectOp(tok); ok && i+1 < len(tokens) {
			r := fileOrNullRedirect(tokens[i+1], op.append)
			if isStdout {
				stdout = r
			} else {
				stderr = r
			}
			i += 2
			continue
		}

		// Single-token with embedded target: ">file", ">>file", "2>file", etc.
		if fd, r, ok := parseSingleRedirectToken(tok); ok {
			if fd == 1 {
				stdout = r
			} else {
				stderr = r
			}
			i++
			continue
		}

		remaining = append(remaining, tok)
		i++
	}
	return
}

type redirectOpSpec struct{ append bool }

func standaloneRedirectOp(tok string) (spec redirectOpSpec, isStdout bool, ok bool) {
	switch tok {
	case ">", "1>":
		return redirectOpSpec{false}, true, true
	case ">>", "1>>":
		return redirectOpSpec{true}, true, true
	case "2>":
		return redirectOpSpec{false}, false, true
	case "2>>":
		return redirectOpSpec{true}, false, true
	}
	return redirectOpSpec{}, false, false
}

// parseSingleRedirectToken handles tokens with the operator and target merged,
// e.g. ">file", ">>file", "2>file", ">/dev/null". Returns fd (1 or 2).
func parseSingleRedirectToken(tok string) (fd int, r domain.StreamRedirect, ok bool) {
	type pfx struct {
		s      string
		fd     int
		append bool
	}
	// Longer prefixes must be checked first to avoid ">>" matching as ">".
	for _, p := range []pfx{
		{"1>>", 1, true}, {"2>>", 2, true}, {">>", 1, true},
		{"1>", 1, false}, {"2>", 2, false}, {">", 1, false},
	} {
		if strings.HasPrefix(tok, p.s) {
			target := tok[len(p.s):]
			if target == "" || target == "&1" || target == "&2" {
				return 0, domain.StreamRedirect{}, false
			}
			return p.fd, fileOrNullRedirect(target, p.append), true
		}
	}
	return 0, domain.StreamRedirect{}, false
}

func fileOrNullRedirect(target string, append bool) domain.StreamRedirect {
	if target == "/dev/null" {
		return domain.StreamRedirect{Target: domain.RedirectNull}
	}
	return domain.StreamRedirect{Target: domain.RedirectFile, File: target, Append: append}
}
